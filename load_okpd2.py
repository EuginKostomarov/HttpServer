#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Скрипт для загрузки данных ОКПД2 в базу данных
"""

import sqlite3
import re
import sys
import os

def determine_level(code):
    """Определяет уровень вложенности кода ОКПД2"""
    return code.count('.')

def determine_parent_code(code):
    """Определяет родительский код для ОКПД2"""
    last_dot = code.rfind('.')
    if last_dot == -1:
        return ""
    return code[:last_dot]

def parse_descriptions(description_line):
    """Парсит строку описаний формата [C | код] Название"""
    descriptions = {}
    if not description_line:
        return descriptions
    
    # Разбиваем по паттерну [C |
    parts = description_line.split('[C |')
    for i in range(1, len(parts)):
        part = parts[i].strip()
        # Ищем закрывающую скобку ]
        close_bracket = part.find(']')
        if close_bracket > 0:
            code = part[:close_bracket].strip()
            # Название начинается после ]
            name_part = part[close_bracket+1:].strip()
            # Убираем запятую в конце, если следующее описание начинается с [C |
            if name_part.endswith(','):
                name_part = name_part[:-1].strip()
            if name_part:
                descriptions[code] = name_part
    
    return descriptions

def parse_okpd2_from_text(text):
    """Парсит данные ОКПД2 из текстового формата"""
    entries = []
    entry_map = {}  # Для отслеживания уже созданных записей
    
    lines = text.split('\n')
    i = 0
    
    while i < len(lines):
        line = lines[i].strip()
        
        # Пропускаем пустые строки и строки с датами/статусами
        if not line or re.match(r'^\d+\s+\d{2}\.\d{2}\.\d{4}', line):
            i += 1
            continue
        
        # Проверяем, является ли строка названием категории (не содержит коды)
        if not re.search(r'\d+\.\d+', line) and line and '[C |' not in line:
            category_name = line
            i += 1
            
            # Ищем следующую строку с кодами
            code_line = ""
            description_line = ""
            
            while i < len(lines):
                next_line = lines[i].strip()
                
                # Если это пустая строка, пропускаем
                if not next_line:
                    i += 1
                    continue
                
                # Если это строка с датами/статусами, пропускаем
                if re.match(r'^\d+\s+\d{2}\.\d{2}\.\d{4}', next_line):
                    i += 1
                    continue
                
                # Проверяем, содержит ли строка коды
                if re.search(r'\d+\.\d+', next_line):
                    # Разделяем по табуляции, если есть
                    parts = next_line.split('\t')
                    if len(parts) >= 1:
                        code_line = parts[0].strip()
                        if len(parts) >= 2:
                            description_line = parts[1].strip()
                    else:
                        code_line = next_line
                    i += 1
                    break
                
                i += 1
            
            if not code_line:
                continue
            
            # Парсим коды (фильтруем даты и другие не-коды)
            codes = []
            for c in code_line.split(','):
                code = c.strip()
                # Проверяем, что это код ОКПД2 (начинается с цифры, содержит точку)
                if code and re.match(r'^\d+\.\d+', code) and not re.match(r'^\d{2}\.\d{2}\.\d{4}', code):
                    codes.append(code)
            # Парсим описания
            descriptions = parse_descriptions(description_line)
            
            # Создаем записи для каждого кода
            for code_str in codes:
                if not code_str:
                    continue
                
                # Ищем описание для этого кода
                name = category_name
                if code_str in descriptions and descriptions[code_str]:
                    name = descriptions[code_str]
                
                # Определяем уровень и родительский код
                level = determine_level(code_str)
                parent_code = determine_parent_code(code_str)
                
                # Проверяем, не создали ли мы уже запись с таким кодом
                if code_str in entry_map:
                    # Обновляем название, если оно более подробное
                    if len(name) > len(entry_map[code_str]['name']):
                        entry_map[code_str]['name'] = name
                else:
                    entry = {
                        'code': code_str,
                        'name': name,
                        'parent_code': parent_code,
                        'level': level
                    }
                    entries.append(entry)
                    entry_map[code_str] = entry
        else:
            i += 1
    
    print(f"Распарсено {len(entries)} записей ОКПД2")
    return entries

def create_okpd2_table(cursor):
    """Создает таблицу okpd2_classifier если её нет"""
    # Проверяем существование таблицы
    cursor.execute("""
        SELECT name FROM sqlite_master 
        WHERE type='table' AND name='okpd2_classifier'
    """)
    if cursor.fetchone():
        return  # Таблица уже существует
    
    # Создаем таблицу
    cursor.execute("""
        CREATE TABLE okpd2_classifier (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            code TEXT NOT NULL UNIQUE,
            name TEXT NOT NULL,
            parent_code TEXT,
            level INTEGER,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    """)
    
    # Создаем индексы
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_okpd2_code ON okpd2_classifier(code)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_okpd2_parent ON okpd2_classifier(parent_code)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_okpd2_level ON okpd2_classifier(level)")

def load_okpd2_to_database(db_path, entries):
    """Загружает записи ОКПД2 в базу данных"""
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    try:
        # Создаем таблицу если её нет
        create_okpd2_table(cursor)
        conn.commit()
        
        # Очищаем таблицу перед загрузкой
        cursor.execute("DELETE FROM okpd2_classifier")
        
        # Вставляем записи
        for entry in entries:
            parent_code = entry['parent_code'] if entry['parent_code'] else None
            cursor.execute("""
                INSERT INTO okpd2_classifier (code, name, parent_code, level)
                VALUES (?, ?, ?, ?)
            """, (entry['code'], entry['name'], parent_code, entry['level']))
        
        conn.commit()
        print(f"Успешно загружено {len(entries)} записей ОКПД2 в базу данных")
        
        # Проверяем количество загруженных записей
        cursor.execute("SELECT COUNT(*) FROM okpd2_classifier")
        count = cursor.fetchone()[0]
        print(f"Всего записей ОКПД2 в базе данных: {count}")
        
        # Выводим несколько примеров
        cursor.execute("SELECT code, name, level FROM okpd2_classifier ORDER BY code LIMIT 10")
        print("\nПримеры загруженных записей:")
        for row in cursor.fetchall():
            print(f"  {row[0]} (уровень {row[2]}): {row[1]}")
        
    except Exception as e:
        conn.rollback()
        raise e
    finally:
        conn.close()

def main():
    if len(sys.argv) < 3:
        print("Использование: python load_okpd2.py <файл_с_данными> <путь_к_базе>")
        sys.exit(1)
    
    file_path = sys.argv[1]
    db_path = sys.argv[2]
    
    if not os.path.exists(file_path):
        print(f"Ошибка: файл {file_path} не найден")
        sys.exit(1)
    
    if not os.path.exists(db_path):
        print(f"Ошибка: база данных {db_path} не найдена")
        sys.exit(1)
    
    # Читаем файл
    with open(file_path, 'r', encoding='utf-8') as f:
        text = f.read()
    
    # Парсим данные
    entries = parse_okpd2_from_text(text)
    
    # Загружаем в базу данных
    load_okpd2_to_database(db_path, entries)

if __name__ == '__main__':
    main()

