#!/usr/bin/env node

/**
 * Скрипт для автоматической миграции API routes на использование getBackendUrl()
 * 
 * Использование:
 *   node scripts/migrate-api-routes.js [--dry-run]
 * 
 * Опции:
 *   --dry-run  - только показать изменения, не применять их
 */

const fs = require('fs')
const path = require('path')
const { glob } = require('glob')

const DRY_RUN = process.argv.includes('--dry-run')
const API_DIR = path.join(__dirname, '../app/api')

// Паттерны для поиска и замены
const PATTERNS = [
  {
    // const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:9999'
    find: /const\s+(BACKEND_URL|API_BASE_URL|API_BASE)\s*=\s*process\.env\.(BACKEND_URL|NEXT_PUBLIC_BACKEND_URL)(?:\s*\|\|\s*['"]?[^'"]+['"]?)?/g,
    replace: (match, varName) => {
      return `import { getBackendUrl } from '@/lib/api-config'\n\nconst ${varName} = getBackendUrl()`
    },
    needsImport: true
  },
  {
    // const BACKEND_URL = process.env.BACKEND_URL || process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:9999'
    find: /const\s+(BACKEND_URL|API_BASE_URL|API_BASE)\s*=\s*process\.env\.BACKEND_URL\s*\|\|\s*process\.env\.NEXT_PUBLIC_BACKEND_URL(?:\s*\|\|\s*['"]?[^'"]+['"]?)?/g,
    replace: (match, varName) => {
      return `import { getBackendUrl } from '@/lib/api-config'\n\nconst ${varName} = getBackendUrl()`
    },
    needsImport: true
  }
]

async function findApiRouteFiles() {
  const files = await glob('**/route.ts', {
    cwd: API_DIR,
    absolute: true
  })
  return files
}

function needsMigration(content) {
  return PATTERNS.some(pattern => pattern.find.test(content))
}

function migrateFile(filePath) {
  let content = fs.readFileSync(filePath, 'utf-8')
  let modified = false
  let hasImport = content.includes("from '@/lib/api-config'")

  for (const pattern of PATTERNS) {
    if (pattern.find.test(content)) {
      if (pattern.needsImport && !hasImport) {
        // Добавляем импорт после существующих импортов
        const importMatch = content.match(/^(import\s+.*?from\s+['"].*?['"];?\s*\n)+/m)
        if (importMatch) {
          const lastImportIndex = importMatch[0].lastIndexOf('\n')
          content = content.slice(0, importMatch.index + lastImportIndex + 1) +
                   "import { getBackendUrl } from '@/lib/api-config'\n" +
                   content.slice(importMatch.index + lastImportIndex + 1)
        } else {
          // Если нет импортов, добавляем в начало
          content = "import { getBackendUrl } from '@/lib/api-config'\n\n" + content
        }
        hasImport = true
      }

      // Заменяем паттерн
      content = content.replace(pattern.find, (match, varName) => {
        return `const ${varName} = getBackendUrl()`
      })
      modified = true
    }
  }

  // Удаляем дублирующиеся импорты
  const importLines = content.match(/^import\s+.*?from\s+['"]@\/lib\/api-config['"];?\s*$/gm)
  if (importLines && importLines.length > 1) {
    const firstImport = importLines[0]
    content = content.replace(new RegExp(`^import\\s+.*?from\\s+['"]@/lib/api-config['"];?\\s*$`, 'gm'), (match, offset) => {
      return offset === content.indexOf(firstImport) ? match : ''
    })
    // Удаляем пустые строки
    content = content.replace(/\n\n\n+/g, '\n\n')
  }

  return { content, modified }
}

async function main() {
  console.log('🔍 Поиск API route файлов...')
  const files = await findApiRouteFiles()
  console.log(`   Найдено ${files.length} файлов\n`)

  const filesToMigrate = []
  for (const file of files) {
    const content = fs.readFileSync(file, 'utf-8')
    if (needsMigration(content)) {
      filesToMigrate.push(file)
    }
  }

  console.log(`📋 Файлов для миграции: ${filesToMigrate.length}\n`)

  if (filesToMigrate.length === 0) {
    console.log('✅ Все файлы уже мигрированы!')
    return
  }

  if (DRY_RUN) {
    console.log('🔍 Режим проверки (dry-run). Файлы, которые будут изменены:\n')
    filesToMigrate.forEach(file => {
      console.log(`   - ${path.relative(API_DIR, file)}`)
    })
    console.log(`\n💡 Запустите без --dry-run для применения изменений`)
    return
  }

  console.log('🚀 Начинаю миграцию...\n')

  let migrated = 0
  let errors = 0

  for (const file of filesToMigrate) {
    try {
      const { content, modified } = migrateFile(file)
      if (modified) {
        fs.writeFileSync(file, content, 'utf-8')
        console.log(`   ✅ ${path.relative(API_DIR, file)}`)
        migrated++
      }
    } catch (error) {
      console.error(`   ❌ ${path.relative(API_DIR, file)}: ${error.message}`)
      errors++
    }
  }

  console.log(`\n✨ Миграция завершена!`)
  console.log(`   Успешно: ${migrated}`)
  if (errors > 0) {
    console.log(`   Ошибок: ${errors}`)
  }
}

main().catch(console.error)

