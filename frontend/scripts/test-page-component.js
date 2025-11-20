#!/usr/bin/env node

/**
 * Скрипт для тестирования компонента страницы проекта
 * Проверяет синтаксис, структуру и использование функций
 */

const fs = require('fs');
const path = require('path');

console.log('🧪 Тестирование компонента страницы проекта...\n');

const pagePath = path.join(__dirname, '..', 'app', 'clients', '[clientId]', 'projects', '[projectId]', 'page.tsx');

if (!fs.existsSync(pagePath)) {
  console.error('❌ Файл не найден:', pagePath);
  process.exit(1);
}

const content = fs.readFileSync(pagePath, 'utf8');
let errors = [];
let warnings = [];

// Проверка 1: Наличие 'use client'
if (!content.includes("'use client'")) {
  errors.push("Отсутствует директива 'use client'");
} else {
  console.log('✅ Директива \'use client\' найдена');
}

// Проверка 2: Наличие экспорта по умолчанию
if (!content.includes('export default function')) {
  errors.push('Отсутствует экспорт по умолчанию');
} else {
  console.log('✅ Экспорт по умолчанию найден');
}

// Проверка 3: Проверка импортов React hooks
const requiredImports = ['useState', 'useEffect', 'useCallback'];
requiredImports.forEach(imp => {
  if (!content.includes(imp)) {
    warnings.push(`Импорт ${imp} не найден`);
  } else {
    console.log(`✅ Импорт ${imp} найден`);
  }
});

// Проверка 4: Проверка определения функций
const functions = [
  'handleFileUpload',
  'handleDrop',
  'handleFileInput',
  'handleDragOver',
  'handleDragLeave',
  'fetchDatabases',
  'fetchProjectDetail',
  'fetchPendingDatabases'
];

console.log('\n📋 Проверка функций:');
functions.forEach(func => {
  const regex = new RegExp(`(const|function)\\s+${func}`, 'g');
  const matches = content.match(regex);
  if (!matches || matches.length === 0) {
    errors.push(`Функция ${func} не найдена`);
  } else if (matches.length > 1) {
    errors.push(`Функция ${func} определена несколько раз (${matches.length})`);
  } else {
    console.log(`   ✅ ${func} определена`);
  }
});

// Проверка 5: Проверка useCallback зависимостей
console.log('\n🔗 Проверка зависимостей useCallback:');

// handleFileUpload должна быть определена до handleDrop
const handleFileUploadPos = content.indexOf('const handleFileUpload');
const handleDropPos = content.indexOf('const handleDrop');
if (handleFileUploadPos === -1 || handleDropPos === -1) {
  errors.push('Не найдены функции handleFileUpload или handleDrop');
} else if (handleFileUploadPos > handleDropPos) {
  errors.push('handleFileUpload должна быть определена до handleDrop');
} else {
  console.log('   ✅ Порядок определения функций правильный');
}

// Проверка 6: Проверка использования handleFileUpload в зависимостях
if (content.includes('handleDrop') && content.includes('handleFileUpload')) {
  const handleDropMatch = content.match(/const handleDrop = useCallback\([^}]+}, \[([^\]]+)\]\)/s);
  if (handleDropMatch && handleDropMatch[1].includes('handleFileUpload')) {
    console.log('   ✅ handleDrop правильно использует handleFileUpload в зависимостях');
  } else {
    warnings.push('handleDrop может не использовать handleFileUpload в зависимостях');
  }
}

// Проверка 7: Проверка синтаксиса useCallback
const useCallbackRegex = /useCallback\([^}]+}, \[([^\]]+)\]\)/g;
let useCallbackCount = 0;
let match;
while ((match = useCallbackRegex.exec(content)) !== null) {
  useCallbackCount++;
  const deps = match[1].trim();
  if (deps === '') {
    warnings.push(`useCallback #${useCallbackCount} имеет пустой массив зависимостей`);
  }
}
console.log(`   ✅ Найдено ${useCallbackCount} использований useCallback`);

// Проверка 8: Проверка закрывающих скобок
const openBraces = (content.match(/{/g) || []).length;
const closeBraces = (content.match(/}/g) || []).length;
if (openBraces !== closeBraces) {
  errors.push(`Несоответствие скобок: открывающих { = ${openBraces}, закрывающих } = ${closeBraces}`);
} else {
  console.log('✅ Скобки сбалансированы');
}

// Проверка 9: Проверка на дубликаты функций
const duplicateCheck = functions.map(func => {
  const regex = new RegExp(`const ${func}\\s*=`, 'g');
  const matches = content.match(regex);
  return { func, count: matches ? matches.length : 0 };
});

const duplicates = duplicateCheck.filter(f => f.count > 1);
if (duplicates.length > 0) {
  duplicates.forEach(d => {
    errors.push(`Функция ${d.func} определена ${d.count} раз(а)`);
  });
} else {
  console.log('✅ Нет дубликатов функций');
}

// Итоговый отчет
console.log('\n' + '='.repeat(50));
if (errors.length === 0 && warnings.length === 0) {
  console.log('✅ Все проверки пройдены успешно!');
  process.exit(0);
} else {
  if (errors.length > 0) {
    console.log(`\n❌ Найдено ${errors.length} ошибок:`);
    errors.forEach(err => console.log(`   - ${err}`));
  }
  if (warnings.length > 0) {
    console.log(`\n⚠️  Найдено ${warnings.length} предупреждений:`);
    warnings.forEach(warn => console.log(`   - ${warn}`));
  }
  process.exit(errors.length > 0 ? 1 : 0);
}

