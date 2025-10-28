#!/bin/bash

# 币安集成测试运行脚本
# 用法: ./run_tests.sh [选项]

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 测试目录
TEST_DIR="./internal/service/exchange/binance/integration"
TIMEOUT="300s"

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 打印标题
print_header() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}$1${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
}

# 检查配置文件
check_config() {
    print_info "检查配置文件..."
    if [ ! -f "config/config.dev.yaml" ]; then
        print_error "配置文件不存在: config/config.dev.yaml"
        exit 1
    fi
    print_success "配置文件检查通过"
}

# 运行测试并统计结果
run_test() {
    local test_name=$1
    local test_pattern=$2
    local description=$3
    
    print_header "$description"
    
    if go test -v $TEST_DIR -run "$test_pattern" -timeout $TIMEOUT; then
        print_success "$test_name 测试通过"
        return 0
    else
        print_error "$test_name 测试失败"
        return 1
    fi
}

# 显示帮助信息
show_help() {
    cat << EOF
币安集成测试运行脚本

用法: ./run_tests.sh [选项]

选项:
    all                 运行所有测试（包括实际交易）
    safe                只运行安全测试（不产生费用）
    order               运行订单服务测试
    trading             运行交易服务测试
    account             运行账户服务测试
    history             运行持仓历史测试
    
    order-safe          运行订单服务安全测试
    trading-safe        运行交易服务安全测试
    
    quick               快速测试（只运行关键测试）
    coverage            生成覆盖率报告
    
    -h, --help          显示此帮助信息

示例:
    ./run_tests.sh safe              # 运行所有安全测试
    ./run_tests.sh order             # 运行订单服务测试
    ./run_tests.sh coverage          # 生成覆盖率报告

测试说明:
    - 交易对: XRP/USDT
    - 测试仓位: 4 XRP (约 10 USDT)
    - 预计总手续费: ~0.05 USDT

EOF
}

# 运行所有安全测试
run_safe_tests() {
    print_header "运行所有安全测试（不产生实际交易费用）"
    
    local failed=0
    
    # 账户服务测试（全部安全）
    run_test "AccountService" "TestAccountServiceSuite" "账户服务测试" || ((failed++))
    
    # 订单服务安全测试（排除 Test07）
    run_test "OrderService(Safe)" "TestOrderServiceSuite/Test0[1-6]" "订单服务测试（安全部分）" || ((failed++))
    
    # 交易服务安全测试（只有 Test01 和 Test02）
    run_test "TradingService(Safe)" "TestTradingServiceSuite/Test0[12]" "交易服务测试（安全部分）" || ((failed++))
    
    # 持仓历史安全测试（排除 Test05）
    run_test "PositionHistory(Safe)" "TestPositionHistorySuite/Test0[1-46]" "持仓历史测试（安全部分）" || ((failed++))
    
    echo ""
    if [ $failed -eq 0 ]; then
        print_success "所有安全测试通过！"
        return 0
    else
        print_error "$failed 个测试套件失败"
        return 1
    fi
}

# 运行所有测试
run_all_tests() {
    print_header "运行所有集成测试（包括实际交易）"
    print_warning "此操作会产生实际交易和手续费（预计约 0.05 USDT）"
    print_warning "使用 XRP/USDT 交易对，4 XRP 仓位"
    
    read -p "确认继续？(y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "已取消"
        exit 0
    fi
    
    local failed=0
    
    run_test "OrderService" "TestOrderServiceSuite" "订单服务完整测试" || ((failed++))
    run_test "TradingService" "TestTradingServiceSuite" "交易服务完整测试" || ((failed++))
    run_test "AccountService" "TestAccountServiceSuite" "账户服务完整测试" || ((failed++))
    run_test "PositionHistory" "TestPositionHistorySuite" "持仓历史完整测试" || ((failed++))
    
    echo ""
    if [ $failed -eq 0 ]; then
        print_success "所有测试通过！"
        return 0
    else
        print_error "$failed 个测试套件失败"
        return 1
    fi
}

# 快速测试
run_quick_tests() {
    print_header "快速测试（只运行关键测试）"
    
    local failed=0
    
    run_test "AccountInfo" "TestAccountServiceSuite/Test01_GetAccountInfo" "账户信息查询" || ((failed++))
    run_test "CreateOrder" "TestOrderServiceSuite/Test01_CreateAndQueryOrder" "订单创建和查询" || ((failed++))
    run_test "RecentHistory" "TestPositionHistorySuite/Test01_GetRecentHistoryPositions" "最近持仓历史" || ((failed++))
    
    echo ""
    if [ $failed -eq 0 ]; then
        print_success "快速测试通过！"
        return 0
    else
        print_error "$failed 个测试失败"
        return 1
    fi
}

# 生成覆盖率报告
generate_coverage() {
    print_header "生成测试覆盖率报告"
    
    print_info "运行测试并收集覆盖率数据..."
    go test $TEST_DIR -run TestAccountServiceSuite -coverprofile=coverage.out -timeout $TIMEOUT
    
    print_info "生成 HTML 报告..."
    go tool cover -html=coverage.out -o coverage.html
    
    print_success "覆盖率报告已生成: coverage.html"
    
    # 显示覆盖率摘要
    print_info "覆盖率摘要:"
    go tool cover -func=coverage.out | tail -n 1
}

# 主函数
main() {
    # 检查配置
    check_config
    
    # 处理命令行参数
    case "${1:-safe}" in
        all)
            run_all_tests
            ;;
        safe)
            run_safe_tests
            ;;
        order)
            run_test "OrderService" "TestOrderServiceSuite" "订单服务完整测试"
            ;;
        order-safe)
            run_test "OrderService(Safe)" "TestOrderServiceSuite/Test0[1-6]" "订单服务安全测试"
            ;;
        trading)
            print_warning "此测试会产生实际交易（约 0.02 USDT 手续费）"
            read -p "确认继续？(y/N): " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                run_test "TradingService" "TestTradingServiceSuite" "交易服务完整测试"
            fi
            ;;
        trading-safe)
            run_test "TradingService(Safe)" "TestTradingServiceSuite/Test0[12]" "交易服务安全测试"
            ;;
        account)
            run_test "AccountService" "TestAccountServiceSuite" "账户服务测试"
            ;;
        history)
            run_test "PositionHistory" "TestPositionHistorySuite" "持仓历史测试"
            ;;
        quick)
            run_quick_tests
            ;;
        coverage)
            generate_coverage
            ;;
        -h|--help)
            show_help
            ;;
        *)
            print_error "未知选项: $1"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

# 运行主函数
main "$@"

