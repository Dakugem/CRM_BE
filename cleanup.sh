#!/bin/bash
# cleanup.sh — освобождение места на Ubuntu сервере

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${GREEN}[+]${NC} $1"; }
info() { echo -e "${CYAN}[i]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }

wait_for_apt() {
    local i=0
    while fuser /var/lib/apt/lists/lock /var/lib/dpkg/lock /var/lib/dpkg/lock-frontend &>/dev/null; do
        if [ $i -eq 0 ]; then
            warn "APT заблокирован другим процессом, ожидаем..."
        fi
        sleep 3
        i=$((i+1))
        if [ $i -ge 20 ]; then
            warn "APT занят слишком долго, пропускаем очистку пакетов."
            return 1
        fi
    done
    return 0
}

free_before=$(df / --output=avail -BM | tail -1 | tr -d 'M')

echo ""
echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}   Ubuntu Disk Cleanup Script${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

info "Место до очистки:"
df -h /
echo ""

# 1. APT кэш
log "Очистка APT кэша..."
if wait_for_apt; then
    apt-get autoremove -y -q
    apt-get autoclean -q
    apt-get clean -q
fi
echo ""

# 2. Journald логи
log "Очистка системных логов (journald)..."
journalctl --disk-usage 2>/dev/null || true
journalctl --vacuum-time=7d -q 2>/dev/null || true
journalctl --vacuum-size=100M -q 2>/dev/null || true
echo ""

# 3. Старые ротированные логи в /var/log
log "Удаление старых ротированных логов..."
find /var/log -type f \( -name "*.gz" -o -name "*.1" -o -name "*.2" -o -name "*.old" \) -delete 2>/dev/null || true
echo ""

# 4. Временные файлы
log "Очистка временных файлов..."
rm -rf /tmp/* 2>/dev/null || true
rm -rf /var/tmp/* 2>/dev/null || true
echo ""

# 5. Кэш thumbnail и pip
log "Очистка пользовательских кэшей..."
rm -rf /root/.cache/pip 2>/dev/null || true
rm -rf /home/*/.cache/pip 2>/dev/null || true
rm -rf /root/.cache/thumbnails 2>/dev/null || true
rm -rf /home/*/.cache/thumbnails 2>/dev/null || true
echo ""

# 6. Docker (если установлен)
if command -v docker &>/dev/null; then
    log "Очистка Docker..."
    docker system prune -f --volumes 2>/dev/null || true
    echo ""
else
    info "Docker не установлен, пропускаем."
    echo ""
fi

# 7. Python .pyc и __pycache__
log "Удаление Python кэша (__pycache__, .pyc)..."
find /home /root /srv /opt -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
find /home /root /srv /opt -type f -name "*.pyc" -delete 2>/dev/null || true
echo ""

# 8. Большие файлы — только отчёт
echo -e "${CYAN}========================================${NC}"
info "Топ 10 самых больших файлов (>50MB):"
find / -xdev -type f -size +50M 2>/dev/null | xargs du -sh 2>/dev/null | sort -rh | head -10
echo ""

# 9. Итог
free_after=$(df / --output=avail -BM | tail -1 | tr -d 'M')
freed=$((free_after - free_before))

echo -e "${CYAN}========================================${NC}"
info "Место после очистки:"
df -h /
echo ""
echo -e "${GREEN}Освобождено: ~${freed} MB${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""
