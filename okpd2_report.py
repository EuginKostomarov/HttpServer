#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Отчет по загруженным данным ОКПД2
"""

import sqlite3
import sys

def generate_report(db_path):
    """Генерирует отчет по загруженным данным"""
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    print("=" * 80)
    print("ОТЧЕТ ПО ЗАГРУЖЕННЫМ ДАННЫМ ОКПД2")
    print("=" * 80)
    
    # Общая статистика
    cursor.execute("SELECT COUNT(*) FROM okpd2_classifier")
    total = cursor.fetchone()[0]
    print(f"\nВсего записей: {total}")
    
    # Распределение по уровням
    cursor.execute("SELECT level, COUNT(*) FROM okpd2_classifier GROUP BY level ORDER BY level")
    print("\nРаспределение по уровням:")
    for row in cursor.fetchall():
        print(f"  Уровень {row[0]}: {row[1]} записей")
    
    # Группировка по основным категориям (первые 2 сегмента кода)
    cursor.execute("""
        SELECT 
            SUBSTR(code, 1, INSTR(SUBSTR(code, INSTR(code, '.') + 1), '.') + INSTR(code, '.') - 1) as category,
            COUNT(*) as count
        FROM okpd2_classifier
        WHERE code LIKE '26.%'
        GROUP BY category
        ORDER BY category
    """)
    
    print("\nГруппировка по категориям (26.XX):")
    categories = []
    for row in cursor.fetchall():
        category = row[0] if row[0] else "Другие"
        count = row[1]
        categories.append((category, count))
        print(f"  {category}: {count} записей")
    
    # Все записи с группировкой
    print("\n" + "=" * 80)
    print("ВСЕ ЗАГРУЖЕННЫЕ ЗАПИСИ (сгруппированные по категориям):")
    print("=" * 80)
    
    cursor.execute("SELECT code, name, level FROM okpd2_classifier ORDER BY code")
    all_records = cursor.fetchall()
    
    current_category = None
    for code, name, level in all_records:
        # Определяем категорию (первые два сегмента)
        parts = code.split('.')
        if len(parts) >= 2:
            category = f"{parts[0]}.{parts[1]}"
        else:
            category = parts[0]
        
        if category != current_category:
            if current_category is not None:
                print()
            print(f"\n[{category}]")
            current_category = category
        
        indent = "  " * (level - 1)
        name_short = name[:70] + "..." if len(name) > 70 else name
        print(f"{indent}{code} (уровень {level}): {name_short}")
    
    # Проверка наличия всех основных категорий из запроса
    print("\n" + "=" * 80)
    print("ПРОВЕРКА НАЛИЧИЯ КАТЕГОРИЙ:")
    print("=" * 80)
    
    expected_categories = [
        "26.11.1", "26.11.2", "26.11.11", "26.11.12", "26.11.21", "26.11.22",
        "26.12.1", "26.12.2", "26.12.10", "26.12.20",
        "26.20.11", "26.20.12", "26.20.13", "26.20.14", "26.20.15",
        "26.20.16", "26.20.17", "26.20.18", "26.20.2", "26.20.21", "26.20.22",
        "26.20.3", "26.20.30", "26.20.4", "26.20.40"
    ]
    
    cursor.execute("SELECT code FROM okpd2_classifier")
    loaded_codes = set(row[0] for row in cursor.fetchall())
    
    print("\nПроверка основных категорий:")
    missing = []
    for cat in expected_categories:
        if cat in loaded_codes:
            print(f"  ✓ {cat} - загружен")
        else:
            print(f"  ✗ {cat} - НЕ найден")
            missing.append(cat)
    
    if missing:
        print(f"\n⚠ ВНИМАНИЕ: Не найдено {len(missing)} категорий!")
    else:
        print("\n✓ Все основные категории загружены!")
    
    conn.close()

if __name__ == '__main__':
    db_path = sys.argv[1] if len(sys.argv) > 1 else 'data/service.db'
    generate_report(db_path)

