import * as XLSX from 'xlsx'

export interface NormalizationGroup {
  normalized_name: string
  normalized_reference: string
  category: string
  merged_count: number
  avg_confidence?: number
  processing_level?: string
  kpved_code?: string
  kpved_name?: string
  kpved_confidence?: number
  last_normalized_at?: string
}

/**
 * Экспорт групп нормализации в CSV
 */
export const exportGroupsToCSV = (groups: NormalizationGroup[], databaseName?: string) => {
  const lines: string[] = []
  
  // Заголовок
  lines.push('Группы нормализации')
  if (databaseName) {
    lines.push(`База данных: ${databaseName}`)
  }
  lines.push(`Экспортировано: ${new Date().toLocaleDateString('ru-RU')}`)
  lines.push('')
  
  // Заголовки таблицы
  lines.push('Нормализованное название,Нормализованный reference,Категория,Количество объединений,Средняя уверенность,Уровень обработки,КПВЭД код,КПВЭД название,Уверенность КПВЭД,Последняя нормализация')
  
  // Данные
  groups.forEach((group) => {
    const normalizedName = (group.normalized_name || '').replace(/"/g, '""')
    const normalizedRef = (group.normalized_reference || '').replace(/"/g, '""')
    const category = (group.category || '').replace(/"/g, '""')
    const kpvedName = (group.kpved_name || '').replace(/"/g, '""')
    const avgConfidence = group.avg_confidence ? `${(group.avg_confidence * 100).toFixed(1)}%` : ''
    const kpvedConfidence = group.kpved_confidence ? `${(group.kpved_confidence * 100).toFixed(1)}%` : ''
    const lastNormalized = group.last_normalized_at ? new Date(group.last_normalized_at).toLocaleDateString('ru-RU') : ''
    
    lines.push(
      `"${normalizedName}","${normalizedRef}","${category}",${group.merged_count},${avgConfidence},${group.processing_level || ''},${group.kpved_code || ''},"${kpvedName}",${kpvedConfidence},${lastNormalized}`
    )
  })
  
  // Создаём CSV файл
  const csvContent = lines.join('\n')
  const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' }) // BOM для корректного отображения кириллицы
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  const fileName = databaseName 
    ? `normalization_groups_${databaseName}_${new Date().toISOString().split('T')[0]}.csv`
    : `normalization_groups_${new Date().toISOString().split('T')[0]}.csv`
  link.download = fileName
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/**
 * Экспорт групп нормализации в JSON
 */
export const exportGroupsToJSON = (groups: NormalizationGroup[], databaseName?: string) => {
  const exportData = {
    metadata: {
      database: databaseName || 'unknown',
      exported_at: new Date().toISOString(),
      exported_date: new Date().toLocaleDateString('ru-RU'),
      total_groups: groups.length,
    },
    groups: groups.map((group) => ({
      ...group,
      avg_confidence_percent: group.avg_confidence ? `${(group.avg_confidence * 100).toFixed(1)}%` : null,
      kpved_confidence_percent: group.kpved_confidence ? `${(group.kpved_confidence * 100).toFixed(1)}%` : null,
      last_normalized_at_formatted: group.last_normalized_at 
        ? new Date(group.last_normalized_at).toLocaleDateString('ru-RU') 
        : null,
    })),
  }
  
  const jsonContent = JSON.stringify(exportData, null, 2)
  const blob = new Blob([jsonContent], { type: 'application/json;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  const fileName = databaseName 
    ? `normalization_groups_${databaseName}_${new Date().toISOString().split('T')[0]}.json`
    : `normalization_groups_${new Date().toISOString().split('T')[0]}.json`
  link.download = fileName
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/**
 * Экспорт групп нормализации в Excel
 */
export const exportGroupsToExcel = (groups: NormalizationGroup[], databaseName?: string) => {
  const workbook = XLSX.utils.book_new()
  
  // Подготовка данных
  const data = [
    ['Нормализованное название', 'Нормализованный reference', 'Категория', 'Количество объединений', 'Средняя уверенность', 'Уровень обработки', 'КПВЭД код', 'КПВЭД название', 'Уверенность КПВЭД', 'Последняя нормализация'],
    ...groups.map((group) => [
      group.normalized_name || '',
      group.normalized_reference || '',
      group.category || '',
      group.merged_count || 0,
      group.avg_confidence ? `${(group.avg_confidence * 100).toFixed(1)}%` : '',
      group.processing_level || '',
      group.kpved_code || '',
      group.kpved_name || '',
      group.kpved_confidence ? `${(group.kpved_confidence * 100).toFixed(1)}%` : '',
      group.last_normalized_at ? new Date(group.last_normalized_at).toLocaleDateString('ru-RU') : '',
    ]),
  ]
  
  const worksheet = XLSX.utils.aoa_to_sheet(data)
  
  // Установка ширины колонок
  const columnWidths = [
    { wch: 40 }, // Нормализованное название
    { wch: 30 }, // Нормализованный reference
    { wch: 20 }, // Категория
    { wch: 20 }, // Количество объединений
    { wch: 20 }, // Средняя уверенность
    { wch: 20 }, // Уровень обработки
    { wch: 15 }, // КПВЭД код
    { wch: 40 }, // КПВЭД название
    { wch: 20 }, // Уверенность КПВЭД
    { wch: 20 }, // Последняя нормализация
  ]
  worksheet['!cols'] = columnWidths
  
  XLSX.utils.book_append_sheet(workbook, worksheet, 'Группы нормализации')
  
  // Сохраняем файл
  const fileName = databaseName 
    ? `normalization_groups_${databaseName}_${new Date().toISOString().split('T')[0]}.xlsx`
    : `normalization_groups_${new Date().toISOString().split('T')[0]}.xlsx`
  XLSX.writeFile(workbook, fileName)
}

