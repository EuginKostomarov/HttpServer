#!/usr/bin/env node

/**
 * Скрипт для массового обновления API routes
 * Обновляет все файлы, которые используют process.env.BACKEND_URL
 */

const fs = require('fs')
const path = require('path')

const API_DIR = path.join(__dirname, '../app/api')

// Список файлов для обновления
const filesToUpdate = [
  // Clients
  'clients/[clientId]/projects/[projectId]/route.ts',
  'clients/[clientId]/projects/route.ts',
  'clients/[clientId]/route.ts',
  'clients/route.ts',
  
  // Databases
  'databases/find-project/route.ts',
  'databases/pending/[id]/route.ts',
  'databases/pending/[id]/[action]/route.ts',
  'databases/analytics/[dbname]/route.ts',
  'databases/history/[dbname]/route.ts',
  'database/info/route.ts',
  'database/switch/route.ts',
  
  // Quality
  'quality/analyze/route.ts',
  'quality/analyze/status/route.ts',
  'quality/duplicates/route.ts',
  'quality/duplicates/[groupId]/merge/route.ts',
  'quality/violations/route.ts',
  'quality/violations/[violationId]/route.ts',
  'quality/suggestions/route.ts',
  'quality/suggestions/[suggestionId]/apply/route.ts',
  'quality/stats/route.ts',
  
  // Normalization
  'normalization/start/route.ts',
  'normalization/stop/route.ts',
  'normalization/stats/route.ts',
  'normalization/config/route.ts',
  'normalization/databases/route.ts',
  'normalization/tables/route.ts',
  'normalization/columns/route.ts',
  'normalization/groups/route.ts',
  'normalization/group-items/route.ts',
  'normalization/item-attributes/[id]/route.ts',
  'normalization/export-group/route.ts',
  'normalization/pipeline/stats/route.ts',
  
  // KPVED
  'kpved/load/route.ts',
  'kpved/search/route.ts',
  'kpved/hierarchy/route.ts',
  'kpved/stats/route.ts',
  'kpved/current-tasks/route.ts',
  'kpved/reclassify-hierarchical/route.ts',
  
  // Classification
  'classification/classifiers/route.ts',
  'classification/classifiers/by-project-type/route.ts',
  
  // Reclassification
  'reclassification/start/route.ts',
  'reclassification/status/route.ts',
  'reclassification/stop/route.ts',
  
  // Monitoring
  'monitoring/events/route.ts',
  'monitoring/history/route.ts',
  
  // Other
  'pipeline/stats/route.ts',
  'workers/models/route.ts',
  'workers/providers/route.ts',
  'workers/arliai/status/route.ts',
  '1c/processing/xml/route.ts',
]

function updateFile(filePath) {
  try {
    let content = fs.readFileSync(filePath, 'utf-8')
    let modified = false
    
    // Проверяем, нужно ли обновление
    if (!content.includes("from '@/lib/api-config'")) {
      // Паттерн 1: const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:9999'
      if (content.match(/const\s+(BACKEND_URL|API_BASE_URL|API_BASE)\s*=\s*process\.env\.(BACKEND_URL|NEXT_PUBLIC_BACKEND_URL)/)) {
        // Добавляем импорт
        const importMatch = content.match(/^(import\s+.*?from\s+['"].*?['"];?\s*\n)+/m)
        if (importMatch) {
          const lastImportIndex = importMatch[0].lastIndexOf('\n')
          content = content.slice(0, importMatch.index + lastImportIndex + 1) +
                   "import { getBackendUrl } from '@/lib/api-config'\n" +
                   content.slice(importMatch.index + lastImportIndex + 1)
        } else {
          content = "import { getBackendUrl } from '@/lib/api-config'\n\n" + content
        }
        
        // Заменяем объявление
        content = content.replace(
          /const\s+(BACKEND_URL|API_BASE_URL|API_BASE)\s*=\s*process\.env\.(BACKEND_URL|NEXT_PUBLIC_BACKEND_URL)(?:\s*\|\|\s*process\.env\.(BACKEND_URL|NEXT_PUBLIC_BACKEND_URL))?(?:\s*\|\|\s*['"]?[^'"]+['"]?)?/g,
          (match, varName) => `const ${varName} = getBackendUrl()`
        )
        
        modified = true
      }
    }
    
    if (modified) {
      fs.writeFileSync(filePath, content, 'utf-8')
      return true
    }
    return false
  } catch (error) {
    console.error(`Error updating ${filePath}:`, error.message)
    return false
  }
}

function main() {
  console.log('🚀 Начинаю массовое обновление API routes...\n')
  
  let updated = 0
  let skipped = 0
  let errors = 0
  
  for (const file of filesToUpdate) {
    const filePath = path.join(API_DIR, file)
    
    if (!fs.existsSync(filePath)) {
      console.log(`   ⚠️  Файл не найден: ${file}`)
      skipped++
      continue
    }
    
    if (updateFile(filePath)) {
      console.log(`   ✅ ${file}`)
      updated++
    } else {
      console.log(`   ⏭️  ${file} (уже обновлен или не требует изменений)`)
      skipped++
    }
  }
  
  console.log(`\n✨ Обновление завершено!`)
  console.log(`   Обновлено: ${updated}`)
  console.log(`   Пропущено: ${skipped}`)
  if (errors > 0) {
    console.log(`   Ошибок: ${errors}`)
  }
}

main()

