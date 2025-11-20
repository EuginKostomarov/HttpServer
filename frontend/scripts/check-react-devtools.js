#!/usr/bin/env node

/**
 * Скрипт для проверки настроек React DevTools
 * Запуск: node scripts/check-react-devtools.js
 */

const fs = require('fs');
const path = require('path');

console.log('🔍 Проверка настроек React DevTools для Next.js...\n');

// Проверка режима запуска
const nodeEnv = process.env.NODE_ENV || 'development';
const isProduction = nodeEnv === 'production';
const nextDir = path.join(__dirname, '..', '.next');
const hasBuild = fs.existsSync(nextDir) && fs.existsSync(path.join(nextDir, 'BUILD_ID'));

console.log('🔧 Режим запуска:');
if (isProduction || hasBuild) {
  console.log('   ⚠️  Обнаружен production режим!');
  console.log('   ⚠️  React DevTools работает лучше в development режиме');
  console.log('   💡 Запустите: npm run dev (вместо npm start)');
} else {
  console.log('   ✅ Development режим (рекомендуется для DevTools)');
}
console.log(`   NODE_ENV: ${nodeEnv}`);

// Проверка версии React
const packageJsonPath = path.join(__dirname, '..', 'package.json');
const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));

console.log('\n📦 Версии пакетов:');
console.log(`   React: ${packageJson.dependencies.react || 'не найден'}`);
console.log(`   React DOM: ${packageJson.dependencies['react-dom'] || 'не найден'}`);
console.log(`   Next.js: ${packageJson.dependencies.next || 'не найден'}`);

// Проверка конфигурации Next.js
const nextConfigPath = path.join(__dirname, '..', 'next.config.ts');
if (fs.existsSync(nextConfigPath)) {
  console.log('\n✅ next.config.ts найден');
  const config = fs.readFileSync(nextConfigPath, 'utf8');
  
  // Проверка на наличие проблемных настроек
  if (config.includes('output: \'standalone\'')) {
    console.log('⚠️  Обнаружен output: \'standalone\' - это может влиять на DevTools');
  }
  
  if (!config.includes('reactStrictMode')) {
    console.log('💡 Рекомендуется добавить reactStrictMode: true в next.config.ts');
  }
} else {
  console.log('\n⚠️  next.config.ts не найден');
}

// Проверка наличия клиентских компонентов
const appDir = path.join(__dirname, '..', 'app');
if (fs.existsSync(appDir)) {
  console.log('\n📁 Проверка структуры приложения...');
  
  const checkClientComponents = (dir) => {
    const files = fs.readdirSync(dir, { withFileTypes: true });
    let hasClientComponents = false;
    
    for (const file of files) {
      const filePath = path.join(dir, file.name);
      
      if (file.isDirectory()) {
        checkClientComponents(filePath);
      } else if (file.name.endsWith('.tsx') || file.name.endsWith('.jsx')) {
        const content = fs.readFileSync(filePath, 'utf8');
        if (content.includes("'use client'")) {
          hasClientComponents = true;
          console.log(`   ✅ Найден клиентский компонент: ${path.relative(appDir, filePath)}`);
        }
      }
    }
    
    return hasClientComponents;
  };
  
  const hasClient = checkClientComponents(appDir);
  if (!hasClient) {
    console.log('⚠️  Не найдено клиентских компонентов (с \'use client\')');
    console.log('   React DevTools работает только с клиентскими компонентами');
  }
}

// Рекомендации
console.log('\n📋 Рекомендации:');
console.log('   ⚠️  ВАЖНО: Запускайте приложение в development режиме!');
console.log('      ✅ npm run dev (development - полная поддержка DevTools)');
console.log('      ❌ npm start (production - ограниченная поддержка)');
console.log('   1. Убедитесь, что используете Chrome v102+');
console.log('   2. Проверьте, что расширение React DevTools установлено и активно');
console.log('   3. Перезапустите service worker расширения, если вкладка не появляется');
console.log('   4. Очистите кэш Next.js: rm -rf .next');

console.log('\n✨ Проверка завершена!');
console.log('   Для подробной информации см. REACT_DEVTOOLS_SETUP.md\n');

