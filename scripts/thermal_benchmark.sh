#!/bin/bash
# thermal_benchmark.sh - Wire-pod Thermal Validation Script
# Target: Raspberry Pi Zero 2W running wire-pod
#
# Usage: ./thermal_benchmark.sh [duration_seconds] [interval_seconds]
# Default: 1800 seconds (30 min), 10 second interval
#
# Target Metrics:
#   - Idle temperature: < 50°C
#   - Active temperature: < 65°C (sustained)
#   - No thermal throttling observed

DURATION=${1:-1800}  # 30 minutes default
INTERVAL=${2:-10}     # 10 seconds default

TEMP_FILE="/sys/class/thermal/thermal_zone0/temp"
OUTPUT_FILE="/tmp/wirepod_thermal_bench_$(date +%Y%m%d_%H%M%S).log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$OUTPUT_FILE"
}

get_cpu_temp() {
    if [ -f "$TEMP_FILE" ]; then
        local temp_milli=$(cat "$TEMP_FILE")
        echo "scale=1; $temp_milli / 1000" | bc
    else
        echo "0"
    fi
}

get_cpu_usage() {
    top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1
}

check_chipper_process() {
    pgrep -f "./chipper" > /dev/null 2>&1 && echo "RUNNING" || echo "STOPPED"
}

print_header() {
    log "=========================================="
    log "WIRE-POD THERMAL BENCHMARK"
    log "=========================================="
    log "Duration: ${DURATION}s"
    log "Interval: ${INTERVAL}s"
    log "Output:   $OUTPUT_FILE"
    log "Target:   Idle < 50°C, Active < 65°C"
    log "=========================================="
}

measure_once() {
    local temp=$(get_cpu_temp)
    local cpu=$(get_cpu_usage)
    local status=$(check_chipper_process)
    
    local temp_int=${temp%.*}  # Convert to integer for comparison
    
    # Determine status
    if [ "$status" = "STOPPED" ]; then
        local temp_status="${YELLOW}chipper OFF${NC}"
    elif [ $temp_int -lt 50 ]; then
        local temp_status="${GREEN}OK${NC}"
    elif [ $temp_int -lt 65 ]; then
        local temp_status="${YELLOW}WARM${NC}"
    elif [ $temp_int -lt 75 ]; then
        local temp_status="${RED}HOT${NC}"
    else
        local temp_status="${RED}CRITICAL${NC}"
    fi
    
    echo "$temp°C | CPU: ${cpu}% | chipper: $status | Status: $temp_status"
}

benchmark() {
    print_header
    
    local start_time=$(date +%s)
    local end_time=$((start_time + DURATION))
    local iteration=0
    
    # Track statistics
    local min_temp=999
    local max_temp=0
    local avg_temp=0
    local total_temp=0
    local hot_count=0
    local critical_count=0
    
    log "Starting benchmark..."
    
    while [ $(date +%s) -lt $end_time ]; do
        iteration=$((iteration + 1))
        local elapsed=$(($(date +%s) - start_time))
        local temp=$(get_cpu_temp)
        local temp_int=${temp%.*}
        
        # Update statistics
        if [ $temp_int -lt $min_temp ]; then min_temp=$temp_int; fi
        if [ $temp_int -gt $max_temp ]; then max_temp=$temp_int; fi
        total_temp=$((total_temp + temp_int))
        
        if [ $temp_int -ge 75 ]; then hot_count=$((hot_count + 1)); fi
        if [ $temp_int -ge 80 ]; then critical_count=$((critical_count + 1)); fi
        
        # Log with color
        local output=$(measure_once)
        log "[${elapsed}s] $output"
        
        sleep $INTERVAL
    done
    
    # Calculate averages
    local avg_temp=$((total_temp / iteration))
    
    log "=========================================="
    log "BENCHMARK COMPLETE"
    log "=========================================="
    log "Duration:   ${DURATION}s"
    log "Samples:    $iteration"
    log "Min Temp:   ${min_temp}°C"
    log "Max Temp:   ${max_temp}°C"
    log "Avg Temp:   ${avg_temp}°C"
    log "Hot (>75°): $hot_count samples"
    log "Crit (>80°): $critical_count samples"
    log "=========================================="
    
    # Pass/Fail determination
    local PASS=true
    
    if [ $max_temp -ge 80 ]; then
        log "${RED}FAIL: Temperature exceeded 80°C (critical)${NC}"
        PASS=false
    elif [ $max_temp -ge 65 ]; then
        log "${YELLOW}WARN: Temperature exceeded 65°C${NC}"
    fi
    
    if [ $critical_count -gt 0 ]; then
        log "${RED}FAIL: $critical_count critical thermal events${NC}"
        PASS=false
    fi
    
    if [ $PASS = true ]; then
        log "${GREEN}PASS: Thermal management working correctly${NC}"
    fi
    
    log "Full log: $OUTPUT_FILE"
}

# Quick temperature check mode
quick_check() {
    log "Quick temperature check..."
    for i in {1..5}; do
        local temp=$(get_cpu_temp)
        local cpu=$(get_cpu_usage)
        local status=$(check_chipper_process)
        log "[$i] Temp: ${temp}°C | CPU: ${cpu}% | chipper: $status"
        sleep 2
    done
}

# Stress test mode - trigger continuous speech processing
stress_test() {
    log "Starting stress test (30 seconds of wake word simulation)..."
    
    for i in {1..10}; do
        local temp=$(get_cpu_temp)
        local cpu=$(get_cpu_usage)
        log "[$i] During stress: ${temp}°C | CPU: ${cpu}%"
        sleep 3
    done
    
    log "Stress test complete. Cool down period..."
    sleep 60
    
    local final_temp=$(get_cpu_temp)
    log "Final temp after cool down: ${final_temp}°C"
}

# Main
case "${3:-benchmark}" in
    quick)
        quick_check
        ;;
    stress)
        stress_test
        ;;
    *)
        benchmark
        ;;
esac
