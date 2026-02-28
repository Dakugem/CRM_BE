пimport paramiko
import psycopg2
from psycopg2 import sql
from datetime import datetime
import socket
import threading
import time

# ==============================
# НАСТРОЙКИ
# ==============================
SSH_HOST = ''
SSH_PORT = 0000
SSH_USER = ''
SSH_PASSWORD = ''

DB_USER = 'postgres_god'
DB_PASSWORD = 'postgod'
DB_HOST_TUNNEL = '127.0.0.1'      # через туннель
LOCAL_TUNNEL_PORT = 5433           # локальный порт для проброса

# ==============================
# ФУНКЦИИ ПРОБРОСА ПОРТА (из Test.py)
# ==============================
def forward_tunnel(local_port, remote_host, remote_port, ssh_transport):
    """Пробрасывает локальный порт на удалённый через SSH-транспорт."""
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    server.bind(('127.0.0.1', local_port))
    server.listen(5)

    def handler():
        while True:
            local_sock, _ = server.accept()
            channel = ssh_transport.open_channel(
                'direct-tcpip',
                (remote_host, remote_port),
                local_sock.getpeername()
            )
            if channel is None:
                local_sock.close()
                continue

            threading.Thread(target=forward, args=(local_sock, channel), daemon=True).start()
            threading.Thread(target=forward, args=(channel, local_sock), daemon=True).start()

    def forward(src, dst):
        try:
            while True:
                data = src.recv(1024)
                if not data:
                    break
                dst.send(data)
        except:
            pass
        finally:
            src.close()
            dst.close()

    threading.Thread(target=handler, daemon=True).start()
    return server

# ==============================
# ФУНКЦИИ ПОДКЛЮЧЕНИЯ К БД (адаптированы для туннеля)
# ==============================
def get_connection(dbname='test_db'):
    """Возвращает подключение к указанной базе через локальный туннель."""
    return psycopg2.connect(
        host=DB_HOST_TUNNEL,
        port=LOCAL_TUNNEL_PORT,
        user=DB_USER,
        password=DB_PASSWORD,
        dbname=dbname
    )

def connect_company(db_name):
    """Для совместимости – возвращает подключение к базе компании."""
    return get_connection(db_name)

# ==============================
# CRUD ДЛЯ БАЗ ДАННЫХ (без изменений, кроме вызова get_connection)
# ==============================
def create_company_database(company_name):
    conn = get_connection('postgres')   # подключаемся к стандартной базе для создания
    conn.autocommit = True
    cursor = conn.cursor()
    try:
        cursor.execute(sql.SQL("CREATE DATABASE {}").format(
            sql.Identifier(company_name)
        ))
        print(f"[+] БД {company_name} создана")
    except Exception as e:
        print(f"[!] Ошибка создания БД: {e}")
    cursor.close()
    conn.close()

def delete_company_database(company_name):
    conn = get_connection('postgres')
    conn.autocommit = True
    cursor = conn.cursor()
    try:
        cursor.execute(sql.SQL("DROP DATABASE IF EXISTS {}").format(
            sql.Identifier(company_name)
        ))
        print(f"[-] БД {company_name} удалена")
    except Exception as e:
        print(f"[!] Ошибка удаления БД: {e}")
    cursor.close()
    conn.close()

# ==============================
# CRUD ДЛЯ ПЛОЩАДОК (ТАБЛИЦ)
# ==============================
def create_site_table(company_db, site_name):
    conn = connect_company(company_db)
    cursor = conn.cursor()
    try:
        cursor.execute(sql.SQL("""
            CREATE TABLE IF NOT EXISTS {} (
                id SERIAL PRIMARY KEY,
                name VARCHAR(255),
                type VARCHAR(100),
                vendor VARCHAR(100),
                model VARCHAR(100),
                description TEXT,
                ip_address VARCHAR(50),
                created_at TIMESTAMP
            )
        """).format(sql.Identifier(site_name)))
        conn.commit()
        print(f"[+] Площадка {site_name} создана в {company_db}")
    except Exception as e:
        print(f"[!] Ошибка создания площадки: {e}")
    cursor.close()
    conn.close()

def delete_site_table(company_db, site_name):
    conn = connect_company(company_db)
    cursor = conn.cursor()
    try:
        cursor.execute(sql.SQL("DROP TABLE IF EXISTS {}").format(
            sql.Identifier(site_name)
        ))
        conn.commit()
        print(f"[-] Площадка {site_name} удалена")
    except Exception as e:
        print(f"[!] Ошибка удаления площадки: {e}")
    cursor.close()
    conn.close()

# ==============================
# CRUD ДЛЯ ОБОРУДОВАНИЯ
# ==============================
def add_equipment(company_db, site_name, equipment):
    conn = connect_company(company_db)
    cursor = conn.cursor()
    try:
        cursor.execute(sql.SQL("""
            INSERT INTO {} (name, type, vendor, model, description, ip_address, created_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s)
        """).format(sql.Identifier(site_name)), (
            equipment["name"],
            equipment["type"],
            equipment["vendor"],
            equipment["model"],
            equipment["description"],
            equipment["ip_address"],
            datetime.now()
        ))
        conn.commit()
        print(f"[+] Оборудование добавлено в {site_name}")
    except Exception as e:
        print(f"[!] Ошибка добавления оборудования: {e}")
    cursor.close()
    conn.close()

def update_equipment(company_db, site_name, equipment_id, new_description):
    conn = connect_company(company_db)
    cursor = conn.cursor()
    try:
        cursor.execute(sql.SQL("""
            UPDATE {}
            SET description = %s
            WHERE id = %s
        """).format(sql.Identifier(site_name)), (new_description, equipment_id))
        conn.commit()
        print(f"[~] Оборудование обновлено")
    except Exception as e:
        print(f"[!] Ошибка обновления: {e}")
    cursor.close()
    conn.close()

def delete_equipment(company_db, site_name, equipment_id):
    conn = connect_company(company_db)
    cursor = conn.cursor()
    try:
        cursor.execute(sql.SQL("""
            DELETE FROM {}
            WHERE id = %s
        """).format(sql.Identifier(site_name)), (equipment_id,))
        conn.commit()
        print(f"[-] Оборудование удалено")
    except Exception as e:
        print(f"[!] Ошибка удаления: {e}")
    cursor.close()
    conn.close()

# ==============================
# ОСНОВНАЯ ПРОГРАММА
# ==============================
if __name__ == "__main__":
    # 1. Устанавливаем SSH-соединение и туннель
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    try:
        client.connect(
            SSH_HOST, port=SSH_PORT,
            username=SSH_USER, password=SSH_PASSWORD,
            allow_agent=False, look_for_keys=False
        )
        print("[+] SSH соединение установлено")
    except Exception as e:
        print(f"[!] Ошибка SSH подключения: {e}")
        exit(1)

    transport = client.get_transport()
    # Запускаем проброс порта: локальный 5433 -> серверный PostgreSQL (127.0.0.1:5432)
    forward_tunnel(LOCAL_TUNNEL_PORT, '127.0.0.1', 5432, transport)
    time.sleep(1)  # даём время на открытие порта

    # 2. Проверяем доступность PostgreSQL через туннель
    try:
        test_conn = get_connection('postgres')
        test_conn.close()
        print("[+] PostgreSQL доступен через туннель")
    except Exception as e:
        print(f"[!] Не удалось подключиться к PostgreSQL: {e}")
        client.close()
        exit(1)

    # 3. Выполняем все операции
    companies = ["rostelecom", "mts", "beeline"]

    # Создаём базы данных компаний (подключаемся к postgres)
    for company in companies:
        create_company_database(company)

    # Создаём таблицы-площадки в каждой базе
    create_site_table("rostelecom", "site_moscow")
    create_site_table("mts", "site_spb")
    create_site_table("beeline", "site_kazan")

    # Добавляем оборудование
    add_equipment("rostelecom", "site_moscow", {
        "name": "Core Switch 1",
        "type": "Switch",
        "vendor": "Cisco",
        "model": "Cisco Catalyst 9500",
        "description": "Магистральный коммутатор ядра сети",
        "ip_address": "10.0.0.1"
    })

    add_equipment("mts", "site_spb", {
        "name": "Edge Router",
        "type": "Router",
        "vendor": "Juniper",
        "model": "MX480",
        "description": "Пограничный маршрутизатор",
        "ip_address": "10.1.1.1"
    })

    add_equipment("beeline", "site_kazan", {
        "name": "Access Switch",
        "type": "Switch",
        "vendor": "Huawei",
        "model": "S5720",
        "description": "Коммутатор доступа",
        "ip_address": "10.2.2.1"
    })

    print("[+] Все операции успешно завершены")

    # 4. Закрываем туннель и SSH-соединение
    client.close()
    print("[+] SSH туннель закрыт")