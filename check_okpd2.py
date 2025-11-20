#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Скрипт для проверки загруженных данных ОКПД2
"""

import sqlite3
import re
import sys

def check_database(db_path):
    """Проверяет данные в базе данных"""
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    # Общее количество записей
    cursor.execute("SELECT COUNT(*) FROM okpd2_classifier")
    total = cursor.fetchone()[0]
    print(f"Всего записей в БД: {total}")
    
    # Уникальные коды
    cursor.execute("SELECT code FROM okpd2_classifier ORDER BY code")
    codes = [row[0] for row in cursor.fetchall()]
    print(f"Уникальных кодов: {len(codes)}")
    
    # Распределение по уровням
    cursor.execute("SELECT level, COUNT(*) FROM okpd2_classifier GROUP BY level ORDER BY level")
    print("\nРаспределение по уровням:")
    for row in cursor.fetchall():
        print(f"  Уровень {row[0]}: {row[1]} записей")
    
    # Примеры записей
    print("\nПримеры записей (первые 20):")
    cursor.execute("SELECT code, name, level FROM okpd2_classifier ORDER BY code LIMIT 20")
    for row in cursor.fetchall():
        name_short = row[1][:60] + "..." if len(row[1]) > 60 else row[1]
        print(f"  {row[0]} (уровень {row[2]}): {name_short}")
    
    conn.close()
    return codes

def check_file(file_path):
    """Проверяет коды в файле"""
    with open(file_path, 'r', encoding='utf-8') as f:
        text = f.read()
    
    # Ищем все коды ОКПД2
    codes = re.findall(r'\b\d+\.\d+(?:\.\d+)*(?:\.\d+)*\b', text)
    unique_codes = sorted(set(codes))
    
    print(f"\nУникальных кодов в файле: {len(unique_codes)}")
    print("\nПервые 30 кодов из файла:")
    for code in unique_codes[:30]:
        print(f"  {code}")
    
    return unique_codes

def compare(db_codes, file_codes):
    """Сравнивает коды из БД и файла"""
    db_set = set(db_codes)
    file_set = set(file_codes)
    
    only_in_db = db_set - file_set
    only_in_file = file_set - db_set
    
    print(f"\nСравнение:")
    print(f"  Кодов в БД: {len(db_set)}")
    print(f"  Кодов в файле: {len(file_set)}")
    print(f"  Общих кодов: {len(db_set & file_set)}")
    
    if only_in_db:
        print(f"\n  Кодов только в БД (не в файле): {len(only_in_db)}")
        for code in sorted(only_in_db)[:10]:
            print(f"    {code}")
    
    if only_in_file:
        print(f"\n  Кодов только в файле (не в БД): {len(only_in_file)}")
        for code in sorted(only_in_file)[:10]:
            print(f"    {code}")

if __name__ == '__main__':
    db_path = sys.argv[1] if len(sys.argv) > 1 else 'data/service.db'
    file_path = sys.argv[2] if len(sys.argv) > 2 else 'okpd2_full_data.txt'
    
    print("=== Проверка базы данных ===")
    db_codes = check_database(db_path)
    
    print("\n=== Проверка файла ===")
    file_codes = check_file(file_path)
    
    print("\n=== Сравнение ===")
    compare(db_codes, file_codes)

