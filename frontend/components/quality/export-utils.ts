import * as XLSX from 'xlsx'
import jsPDF from 'jspdf'
import 'jspdf-autotable'
import { Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell, WidthType, AlignmentType, HeadingLevel } from 'docx'
import { saveAs } from 'file-saver'

interface QualityReportData {
  generated_at: string
  database: string
  quality_score: number
  summary: {
    total_records: number
    high_quality_records: number
    medium_quality_records: number
    low_quality_records: number
    unique_groups: number
    avg_confidence: number
    success_rate: number
    issues_count: number
    critical_issues: number
  }
  distribution: {
    quality_levels: Array<{
      name: string
      count: number
      percentage: number
    }>
    completed: number
    in_progress: number
    requires_review: number
    failed: number
  }
  detailed: {
    duplicates: Array<any>
    violations: Array<any>
    completeness: Array<any>
    consistency: Array<any>
  }
  recommendations: Array<any>
}

import { normalizePercentage } from '@/lib/locale'

const normalizeQuality = normalizePercentage

export const exportToExcel = (reportData: QualityReportData, databaseName: string) => {
  const workbook = XLSX.utils.book_new()
  
  // Лист "Сводка"
  const summaryData = [
    ['Метрика', 'Значение'],
    ['Общее качество', `${normalizeQuality(reportData.quality_score).toFixed(1)}%`],
    ['Всего записей', reportData.summary.total_records],
    ['Записей высокого качества', reportData.summary.high_quality_records],
    ['Записей среднего качества', reportData.summary.medium_quality_records],
    ['Записей низкого качества', reportData.summary.low_quality_records],
    ['Уникальных групп', reportData.summary.unique_groups],
    ['Средняя уверенность', `${normalizeQuality(reportData.summary.avg_confidence).toFixed(1)}%`],
    ['Процент успеха', `${reportData.summary.success_rate.toFixed(1)}%`],
    ['Количество проблем', reportData.summary.issues_count],
    ['Критических проблем', reportData.summary.critical_issues],
  ]
  
  const summarySheet = XLSX.utils.aoa_to_sheet(summaryData)
  XLSX.utils.book_append_sheet(workbook, summarySheet, 'Сводка')
  
  // Лист "Распределение качества"
  const distributionData = [
    ['Уровень качества', 'Количество записей', 'Процент'],
    ...reportData.distribution.quality_levels.map((level) => [
      level.name,
      level.count,
      `${level.percentage.toFixed(1)}%`
    ])
  ]
  
  const distributionSheet = XLSX.utils.aoa_to_sheet(distributionData)
  XLSX.utils.book_append_sheet(workbook, distributionSheet, 'Распределение качества')
  
  // Лист "Дубликаты"
  if (reportData.detailed.duplicates && reportData.detailed.duplicates.length > 0) {
    const duplicatesData = [
      ['ID', 'Название группы', 'Тип дубликата', 'Количество', 'Схожесть', 'Уверенность', 'Статус'],
      ...reportData.detailed.duplicates.map((dup: any) => [
        dup.id || '',
        dup.group_name || dup.normalized_name || 'Группа ' + (dup.id || dup.group_id || ''),
        dup.duplicate_type_name || dup.duplicate_type || 'Неизвестно',
        dup.count || dup.item_count || 0,
        dup.similarity_score ? `${normalizeQuality(dup.similarity_score).toFixed(1)}%` : '',
        dup.confidence ? `${normalizeQuality(dup.confidence).toFixed(1)}%` : '',
        dup.status === 'resolved' || dup.merged ? 'Объединено' : 'Требует проверки'
      ])
    ]
    const duplicatesSheet = XLSX.utils.aoa_to_sheet(duplicatesData)
    XLSX.utils.book_append_sheet(workbook, duplicatesSheet, 'Дубликаты')
  }
  
  // Лист "Нарушения"
  if (reportData.detailed.violations && reportData.detailed.violations.length > 0) {
    const violationsData = [
      ['ID', 'Тип нарушения', 'Категория', 'Серьёзность', 'Описание', 'Рекомендация', 'Разрешено'],
      ...reportData.detailed.violations.map((violation: any) => [
        violation.id || '',
        violation.type || violation.rule_name || '',
        violation.category || '',
        violation.severity || '',
        violation.message || violation.description || '',
        violation.recommendation || '',
        violation.resolved ? 'Да' : 'Нет'
      ])
    ]
    const violationsSheet = XLSX.utils.aoa_to_sheet(violationsData)
    XLSX.utils.book_append_sheet(workbook, violationsSheet, 'Нарушения')
  }
  
  // Лист "Предложения"
  if (reportData.detailed.completeness && reportData.detailed.completeness.length > 0) {
    const suggestionsData = [
      ['ID', 'Тип', 'Приоритет', 'Поле', 'Текущее значение', 'Предлагаемое значение', 'Уверенность', 'Применено'],
      ...reportData.detailed.completeness.map((suggestion: any) => [
        suggestion.id || '',
        suggestion.type || '',
        suggestion.priority || '',
        suggestion.field || suggestion.field_name || '',
        suggestion.current_value || '',
        suggestion.suggested_value || '',
        suggestion.confidence ? `${normalizeQuality(suggestion.confidence).toFixed(1)}%` : '',
        suggestion.applied ? 'Да' : 'Нет'
      ])
    ]
    const suggestionsSheet = XLSX.utils.aoa_to_sheet(suggestionsData)
    XLSX.utils.book_append_sheet(workbook, suggestionsSheet, 'Предложения')
  }
  
  // Сохраняем файл
  const fileName = `quality_report_${databaseName}_${new Date().toISOString().split('T')[0]}.xlsx`
  XLSX.writeFile(workbook, fileName)
}

export const exportToPDF = (reportData: QualityReportData, databaseName: string) => {
  const doc = new jsPDF()
  
  // Заголовок
  doc.setFontSize(16)
  doc.text('Отчёт оценки качества базы данных', 14, 15)
  doc.setFontSize(12)
  doc.text(`База данных: ${databaseName}`, 14, 25)
  doc.text(`Сгенерировано: ${new Date(reportData.generated_at).toLocaleDateString('ru-RU')}`, 14, 35)
  
  // Сводка
  const summaryData = [
    ['Метрика', 'Значение'],
    ['Общее качество', `${normalizeQuality(reportData.quality_score).toFixed(1)}%`],
    ['Всего записей', reportData.summary.total_records.toString()],
    ['Записей высокого качества', reportData.summary.high_quality_records.toString()],
    ['Записей среднего качества', reportData.summary.medium_quality_records.toString()],
    ['Записей низкого качества', reportData.summary.low_quality_records.toString()],
    ['Уникальных групп', reportData.summary.unique_groups.toString()],
    ['Средняя уверенность', `${normalizeQuality(reportData.summary.avg_confidence).toFixed(1)}%`],
    ['Процент успеха', `${reportData.summary.success_rate.toFixed(1)}%`],
    ['Количество проблем', reportData.summary.issues_count.toString()],
    ['Критических проблем', reportData.summary.critical_issues.toString()],
  ]
  
	;(doc as any).autoTable({
		startY: 45,
		head: [summaryData[0]],
		body: summaryData.slice(1),
		theme: 'striped',
		headStyles: { fillColor: [59, 130, 246] },
	})
  
	// Распределение качества
	const distributionData = [
		['Уровень', 'Количество', 'Процент'],
		...reportData.distribution.quality_levels.map((level) => [
			level.name,
			level.count.toString(),
			`${level.percentage.toFixed(1)}%`
		])
	]
	
	;(doc as any).autoTable({
		startY: 80,
		head: [distributionData[0]],
		body: distributionData.slice(1),
		theme: 'striped',
		headStyles: { fillColor: [16, 185, 129] },
	})
  
  // Сохраняем PDF
  const fileName = `quality_report_${databaseName}_${new Date().toISOString().split('T')[0]}.pdf`
  doc.save(fileName)
}

/**
 * Экспорт в CSV формат
 */
export const exportToCSV = (reportData: QualityReportData, databaseName: string) => {
  const lines: string[] = []
  
  // Заголовок
  lines.push('Отчёт оценки качества базы данных')
  lines.push(`База данных: ${databaseName}`)
  lines.push(`Сгенерировано: ${new Date(reportData.generated_at).toLocaleDateString('ru-RU')}`)
  lines.push('')
  
  // Сводка
  lines.push('=== СВОДКА ===')
  lines.push('Метрика,Значение')
  lines.push(`Общее качество,${normalizeQuality(reportData.quality_score).toFixed(1)}%`)
  lines.push(`Всего записей,${reportData.summary.total_records}`)
  lines.push(`Записей высокого качества,${reportData.summary.high_quality_records}`)
  lines.push(`Записей среднего качества,${reportData.summary.medium_quality_records}`)
  lines.push(`Записей низкого качества,${reportData.summary.low_quality_records}`)
  lines.push(`Уникальных групп,${reportData.summary.unique_groups}`)
  lines.push(`Средняя уверенность,${normalizeQuality(reportData.summary.avg_confidence).toFixed(1)}%`)
  lines.push(`Процент успеха,${reportData.summary.success_rate.toFixed(1)}%`)
  lines.push(`Количество проблем,${reportData.summary.issues_count}`)
  lines.push(`Критических проблем,${reportData.summary.critical_issues}`)
  lines.push('')
  
  // Распределение качества
  lines.push('=== РАСПРЕДЕЛЕНИЕ КАЧЕСТВА ===')
  lines.push('Уровень качества,Количество записей,Процент')
  reportData.distribution.quality_levels.forEach((level) => {
    lines.push(`${level.name},${level.count},${level.percentage.toFixed(1)}%`)
  })
  lines.push('')
  
  // Дубликаты
  if (reportData.detailed.duplicates && reportData.detailed.duplicates.length > 0) {
    lines.push('=== ДУБЛИКАТЫ ===')
    lines.push('ID,Название группы,Тип дубликата,Количество,Схожесть,Уверенность,Статус')
    reportData.detailed.duplicates.forEach((dup: any) => {
      const groupName = dup.group_name || dup.normalized_name || 'Группа ' + (dup.id || dup.group_id || '')
      const duplicateType = dup.duplicate_type_name || dup.duplicate_type || 'Неизвестно'
      const similarity = dup.similarity_score ? `${normalizeQuality(dup.similarity_score).toFixed(1)}%` : ''
      const confidence = dup.confidence ? `${normalizeQuality(dup.confidence).toFixed(1)}%` : ''
      const status = dup.status === 'resolved' || dup.merged ? 'Объединено' : 'Требует проверки'
      lines.push(`${dup.id || ''},"${groupName}",${duplicateType},${dup.count || dup.item_count || 0},${similarity},${confidence},${status}`)
    })
    lines.push('')
  }
  
  // Нарушения
  if (reportData.detailed.violations && reportData.detailed.violations.length > 0) {
    lines.push('=== НАРУШЕНИЯ ===')
    lines.push('ID,Тип нарушения,Категория,Серьёзность,Описание,Рекомендация,Разрешено')
    reportData.detailed.violations.forEach((violation: any) => {
      const message = (violation.message || violation.description || '').replace(/"/g, '""')
      const recommendation = (violation.recommendation || '').replace(/"/g, '""')
      lines.push(`${violation.id || ''},${violation.type || violation.rule_name || ''},${violation.category || ''},${violation.severity || ''},"${message}","${recommendation}",${violation.resolved ? 'Да' : 'Нет'}`)
    })
    lines.push('')
  }
  
  // Предложения
  if (reportData.detailed.completeness && reportData.detailed.completeness.length > 0) {
    lines.push('=== ПРЕДЛОЖЕНИЯ ===')
    lines.push('ID,Тип,Приоритет,Поле,Текущее значение,Предлагаемое значение,Уверенность,Применено')
    reportData.detailed.completeness.forEach((suggestion: any) => {
      const currentValue = (suggestion.current_value || '').replace(/"/g, '""')
      const suggestedValue = (suggestion.suggested_value || '').replace(/"/g, '""')
      const confidence = suggestion.confidence ? `${normalizeQuality(suggestion.confidence).toFixed(1)}%` : ''
      lines.push(`${suggestion.id || ''},${suggestion.type || ''},${suggestion.priority || ''},${suggestion.field || suggestion.field_name || ''},"${currentValue}","${suggestedValue}",${confidence},${suggestion.applied ? 'Да' : 'Нет'}`)
    })
  }
  
  // Создаём CSV файл
  const csvContent = lines.join('\n')
  const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' }) // BOM для корректного отображения кириллицы
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `quality_report_${databaseName}_${new Date().toISOString().split('T')[0]}.csv`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/**
 * Экспорт в JSON формат
 */
export const exportToJSON = (reportData: QualityReportData, databaseName: string) => {
  // Форматируем данные для экспорта
  const exportData = {
    metadata: {
      database: databaseName,
      generated_at: reportData.generated_at,
      generated_date: new Date(reportData.generated_at).toLocaleDateString('ru-RU'),
    },
    summary: {
      quality_score: normalizeQuality(reportData.quality_score),
      quality_score_percent: `${normalizeQuality(reportData.quality_score).toFixed(1)}%`,
      total_records: reportData.summary.total_records,
      high_quality_records: reportData.summary.high_quality_records,
      medium_quality_records: reportData.summary.medium_quality_records,
      low_quality_records: reportData.summary.low_quality_records,
      unique_groups: reportData.summary.unique_groups,
      avg_confidence: normalizeQuality(reportData.summary.avg_confidence),
      avg_confidence_percent: `${normalizeQuality(reportData.summary.avg_confidence).toFixed(1)}%`,
      success_rate: reportData.summary.success_rate,
      issues_count: reportData.summary.issues_count,
      critical_issues: reportData.summary.critical_issues,
    },
    distribution: reportData.distribution,
    detailed: {
      duplicates: reportData.detailed.duplicates.map((dup: any) => ({
        ...dup,
        similarity_score_percent: dup.similarity_score ? `${normalizeQuality(dup.similarity_score).toFixed(1)}%` : null,
        confidence_percent: dup.confidence ? `${normalizeQuality(dup.confidence).toFixed(1)}%` : null,
      })),
      violations: reportData.detailed.violations,
      completeness: reportData.detailed.completeness.map((suggestion: any) => ({
        ...suggestion,
        confidence_percent: suggestion.confidence ? `${normalizeQuality(suggestion.confidence).toFixed(1)}%` : null,
      })),
      consistency: reportData.detailed.consistency,
    },
    recommendations: reportData.recommendations,
  }
  
  const jsonContent = JSON.stringify(exportData, null, 2)
  const blob = new Blob([jsonContent], { type: 'application/json;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `quality_report_${databaseName}_${new Date().toISOString().split('T')[0]}.json`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/**
 * Экспорт в Word (DOCX) формат
 */
export const exportToWord = async (reportData: QualityReportData, databaseName: string) => {
  const children: (Paragraph | Table)[] = []
  
  // Заголовок
  children.push(
    new Paragraph({
      text: 'Отчёт оценки качества базы данных',
      heading: HeadingLevel.HEADING_1,
      alignment: AlignmentType.CENTER,
      spacing: { after: 200 },
    })
  )
  
  children.push(
    new Paragraph({
      children: [
        new TextRun({ text: 'База данных: ', bold: true }),
        new TextRun({ text: databaseName }),
      ],
      spacing: { after: 100 },
    })
  )
  
  children.push(
    new Paragraph({
      children: [
        new TextRun({ text: 'Сгенерировано: ', bold: true }),
        new TextRun({ text: new Date(reportData.generated_at).toLocaleDateString('ru-RU') }),
      ],
      spacing: { after: 300 },
    })
  )
  
  // Сводка
  children.push(
    new Paragraph({
      text: 'Сводка',
      heading: HeadingLevel.HEADING_2,
      spacing: { before: 200, after: 200 },
    })
  )
  
  const summaryTable = new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: [
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Метрика', bold: true })] })] }),
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Значение', bold: true })] })] }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph('Общее качество')] }),
          new TableCell({ children: [new Paragraph(`${normalizeQuality(reportData.quality_score).toFixed(1)}%`)] }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph('Всего записей')] }),
          new TableCell({ children: [new Paragraph(reportData.summary.total_records.toString())] }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph('Записей высокого качества')] }),
          new TableCell({ children: [new Paragraph(reportData.summary.high_quality_records.toString())] }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph('Записей среднего качества')] }),
          new TableCell({ children: [new Paragraph(reportData.summary.medium_quality_records.toString())] }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph('Записей низкого качества')] }),
          new TableCell({ children: [new Paragraph(reportData.summary.low_quality_records.toString())] }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph('Уникальных групп')] }),
          new TableCell({ children: [new Paragraph(reportData.summary.unique_groups.toString())] }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph('Средняя уверенность')] }),
          new TableCell({ children: [new Paragraph(`${normalizeQuality(reportData.summary.avg_confidence).toFixed(1)}%`)] }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph('Процент успеха')] }),
          new TableCell({ children: [new Paragraph(`${reportData.summary.success_rate.toFixed(1)}%`)] }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph('Количество проблем')] }),
          new TableCell({ children: [new Paragraph(reportData.summary.issues_count.toString())] }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph('Критических проблем')] }),
          new TableCell({ children: [new Paragraph(reportData.summary.critical_issues.toString())] }),
        ],
      }),
    ],
  })
  
  children.push(summaryTable)
  children.push(new Paragraph({ text: '', spacing: { after: 300 } }))
  
  // Распределение качества
  children.push(
    new Paragraph({
      text: 'Распределение качества',
      heading: HeadingLevel.HEADING_2,
      spacing: { before: 200, after: 200 },
    })
  )
  
  const distributionTableRows = [
    new TableRow({
      children: [
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Уровень качества', bold: true })] })] }),
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Количество записей', bold: true })] })] }),
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Процент', bold: true })] })] }),
      ],
    }),
    ...reportData.distribution.quality_levels.map((level) =>
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph(level.name)] }),
          new TableCell({ children: [new Paragraph(level.count.toString())] }),
          new TableCell({ children: [new Paragraph(`${level.percentage.toFixed(1)}%`)] }),
        ],
      })
    ),
  ]
  
  const distributionTable = new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: distributionTableRows,
  })
  
  children.push(distributionTable)
  children.push(new Paragraph({ text: '', spacing: { after: 300 } }))
  
  // Дубликаты (ограничиваем до 50 записей для Word)
  if (reportData.detailed.duplicates && reportData.detailed.duplicates.length > 0) {
    children.push(
      new Paragraph({
        text: 'Дубликаты',
        heading: HeadingLevel.HEADING_2,
        spacing: { before: 200, after: 200 },
      })
    )
    
    const duplicatesTableRows = [
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'ID', bold: true })] })] }),
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Название группы', bold: true })] })] }),
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Тип', bold: true })] })] }),
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Количество', bold: true })] })] }),
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Схожесть', bold: true })] })] }),
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Статус', bold: true })] })] }),
        ],
      }),
      ...reportData.detailed.duplicates.slice(0, 50).map((dup: any) => {
        const groupName = dup.group_name || dup.normalized_name || 'Группа ' + (dup.id || dup.group_id || '')
        const duplicateType = dup.duplicate_type_name || dup.duplicate_type || 'Неизвестно'
        const similarity = dup.similarity_score ? `${normalizeQuality(dup.similarity_score).toFixed(1)}%` : '-'
        const status = dup.status === 'resolved' || dup.merged ? 'Объединено' : 'Требует проверки'
        
        return new TableRow({
          children: [
            new TableCell({ children: [new Paragraph((dup.id || '').toString())] }),
            new TableCell({ children: [new Paragraph(groupName)] }),
            new TableCell({ children: [new Paragraph(duplicateType)] }),
            new TableCell({ children: [new Paragraph((dup.count || dup.item_count || 0).toString())] }),
            new TableCell({ children: [new Paragraph(similarity)] }),
            new TableCell({ children: [new Paragraph(status)] }),
          ],
        })
      }),
    ]
    
    if (reportData.detailed.duplicates.length > 50) {
      children.push(
        new Paragraph({
          children: [new TextRun({ text: `Показано 50 из ${reportData.detailed.duplicates.length} дубликатов`, italics: true })],
          spacing: { before: 100 },
        })
      )
    }
    
    const duplicatesTable = new Table({
      width: { size: 100, type: WidthType.PERCENTAGE },
      rows: duplicatesTableRows,
    })
    
    children.push(duplicatesTable)
    children.push(new Paragraph({ text: '', spacing: { after: 300 } }))
  }
  
  // Нарушения (ограничиваем до 50 записей)
  if (reportData.detailed.violations && reportData.detailed.violations.length > 0) {
    children.push(
      new Paragraph({
        text: 'Нарушения',
        heading: HeadingLevel.HEADING_2,
        spacing: { before: 200, after: 200 },
      })
    )
    
    const violationsTableRows = [
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Тип', bold: true })] })] }),
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Категория', bold: true })] })] }),
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Серьёзность', bold: true })] })] }),
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Описание', bold: true })] })] }),
          new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Разрешено', bold: true })] })] }),
        ],
      }),
      ...reportData.detailed.violations.slice(0, 50).map((violation: any) =>
        new TableRow({
          children: [
            new TableCell({ children: [new Paragraph(violation.type || violation.rule_name || '')] }),
            new TableCell({ children: [new Paragraph(violation.category || '')] }),
            new TableCell({ children: [new Paragraph(violation.severity || '')] }),
            new TableCell({ children: [new Paragraph(violation.message || violation.description || '')] }),
            new TableCell({ children: [new Paragraph(violation.resolved ? 'Да' : 'Нет')] }),
          ],
        })
      ),
    ]
    
    if (reportData.detailed.violations.length > 50) {
      children.push(
        new Paragraph({
          children: [new TextRun({ text: `Показано 50 из ${reportData.detailed.violations.length} нарушений`, italics: true })],
          spacing: { before: 100 },
        })
      )
    }
    
    const violationsTable = new Table({
      width: { size: 100, type: WidthType.PERCENTAGE },
      rows: violationsTableRows,
    })
    
    children.push(violationsTable)
    children.push(new Paragraph({ text: '', spacing: { after: 300 } }))
  }
  
  // Рекомендации
  if (reportData.recommendations && reportData.recommendations.length > 0) {
    children.push(
      new Paragraph({
        text: 'Рекомендации',
        heading: HeadingLevel.HEADING_2,
        spacing: { before: 200, after: 200 },
      })
    )
    
    reportData.recommendations.forEach((rec: any, index: number) => {
      children.push(
        new Paragraph({
          children: [
            new TextRun({ text: `${index + 1}. `, bold: true }),
            new TextRun({ text: rec.title || rec.description || rec.message || 'Рекомендация' }),
          ],
          spacing: { after: 100 },
        })
      )
    })
  }
  
  // Создаём документ
  const doc = new Document({
    sections: [
      {
        children,
      },
    ],
  })
  
  // Сохраняем файл
  const blob = await Packer.toBlob(doc)
  const fileName = `quality_report_${databaseName}_${new Date().toISOString().split('T')[0]}.docx`
  saveAs(blob, fileName)
}

