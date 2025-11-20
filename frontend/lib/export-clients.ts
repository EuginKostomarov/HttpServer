/**
 * Утилиты для экспорта данных клиентов
 */

import * as XLSX from 'xlsx'
import jsPDF from 'jspdf'
import 'jspdf-autotable'
import { Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell, WidthType, AlignmentType, HeadingLevel } from 'docx'
import { saveAs } from 'file-saver'
import { getCountryByCode } from './countries'

export interface ClientExportData {
  id: number
  name: string
  legal_name: string
  description: string
  contact_email: string
  contact_phone: string
  tax_id: string
  country?: string
  country_name?: string
  status: string
  project_count: number
  benchmark_count: number
  last_activity: string
}

/**
 * Экспортирует клиентов в CSV формат
 */
export function exportClientsToCSV(clients: ClientExportData[]): void {
  if (clients.length === 0) {
    alert('Нет данных для экспорта')
    return
  }

  // Заголовки
  const headers = [
    'ID',
    'Название',
    'Юридическое название',
    'Описание',
    'Email',
    'Телефон',
    'ИНН/БИН',
    'Страна',
    'Статус',
    'Проектов',
    'Эталонов',
    'Последняя активность'
  ]

  // Данные
  const rows = clients.map(client => [
    client.id,
    client.name || '',
    client.legal_name || '',
    client.description || '',
    client.contact_email || '',
    client.contact_phone || '',
    client.tax_id || '',
    client.country ? (getCountryByCode(client.country)?.name || client.country) : '',
    client.status || '',
    client.project_count || 0,
    client.benchmark_count || 0,
    client.last_activity || ''
  ])

  // Формируем CSV
  const csvContent = [
    headers.join(','),
    ...rows.map(row => 
      row.map(cell => {
        // Экранируем кавычки и запятые
        const cellStr = String(cell || '')
        if (cellStr.includes(',') || cellStr.includes('"') || cellStr.includes('\n')) {
          return `"${cellStr.replace(/"/g, '""')}"`
        }
        return cellStr
      }).join(',')
    )
  ].join('\n')

  // Создаем BOM для корректного отображения кириллицы в Excel
  const BOM = '\uFEFF'
  const blob = new Blob([BOM + csvContent], { type: 'text/csv;charset=utf-8;' })
  
  // Скачиваем файл
  const link = document.createElement('a')
  const url = URL.createObjectURL(blob)
  link.setAttribute('href', url)
  link.setAttribute('download', `clients_${new Date().toISOString().split('T')[0]}.csv`)
  link.style.visibility = 'hidden'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/**
 * Экспортирует клиентов в JSON формат
 */
export function exportClientsToJSON(clients: ClientExportData[]): void {
  if (clients.length === 0) {
    alert('Нет данных для экспорта')
    return
  }

  // Обогащаем данные названиями стран
  const enrichedClients = clients.map(client => ({
    ...client,
    country_name: client.country ? (getCountryByCode(client.country)?.name || client.country) : undefined
  }))

  const jsonContent = JSON.stringify(enrichedClients, null, 2)
  const blob = new Blob([jsonContent], { type: 'application/json;charset=utf-8;' })
  
  const link = document.createElement('a')
  const url = URL.createObjectURL(blob)
  link.setAttribute('href', url)
  link.setAttribute('download', `clients_${new Date().toISOString().split('T')[0]}.json`)
  link.style.visibility = 'hidden'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/**
 * Экспортирует клиентов в Excel формат
 */
export function exportClientsToExcel(clients: ClientExportData[]): void {
  if (clients.length === 0) {
    alert('Нет данных для экспорта')
    return
  }

  const workbook = XLSX.utils.book_new()
  
  // Подготовка данных
  const data = [
    ['ID', 'Название', 'Юридическое название', 'Описание', 'Email', 'Телефон', 'ИНН/БИН', 'Страна', 'Статус', 'Проектов', 'Эталонов', 'Последняя активность'],
    ...clients.map(client => [
      client.id,
      client.name || '',
      client.legal_name || '',
      client.description || '',
      client.contact_email || '',
      client.contact_phone || '',
      client.tax_id || '',
      client.country ? (getCountryByCode(client.country)?.name || client.country) : '',
      client.status || '',
      client.project_count || 0,
      client.benchmark_count || 0,
      client.last_activity || '',
    ]),
  ]
  
  const worksheet = XLSX.utils.aoa_to_sheet(data)
  
  // Установка ширины колонок
  const columnWidths = [
    { wch: 8 },  // ID
    { wch: 30 }, // Название
    { wch: 35 }, // Юридическое название
    { wch: 40 }, // Описание
    { wch: 25 }, // Email
    { wch: 18 }, // Телефон
    { wch: 15 }, // ИНН/БИН
    { wch: 20 }, // Страна
    { wch: 15 }, // Статус
    { wch: 12 }, // Проектов
    { wch: 12 }, // Эталонов
    { wch: 20 }, // Последняя активность
  ]
  worksheet['!cols'] = columnWidths
  
  XLSX.utils.book_append_sheet(workbook, worksheet, 'Клиенты')
  
  // Сохраняем файл
  const fileName = `clients_${new Date().toISOString().split('T')[0]}.xlsx`
  XLSX.writeFile(workbook, fileName)
}

/**
 * Экспортирует клиентов в PDF формат
 */
export function exportClientsToPDF(clients: ClientExportData[]): void {
  if (clients.length === 0) {
    alert('Нет данных для экспорта')
    return
  }

  const doc = new jsPDF()
  
  // Заголовок
  doc.setFontSize(16)
  doc.text('Список клиентов', 14, 15)
  doc.setFontSize(12)
  doc.text(`Экспортировано: ${new Date().toLocaleDateString('ru-RU')}`, 14, 25)
  doc.text(`Всего клиентов: ${clients.length}`, 14, 35)
  
  // Подготовка данных для таблицы
  const tableData = clients.map(client => [
    client.id.toString(),
    client.name || '',
    client.legal_name || '',
    client.contact_email || '',
    client.tax_id || '',
    client.country ? (getCountryByCode(client.country)?.name || client.country) : '',
    client.status || '',
    (client.project_count || 0).toString(),
  ])
  
  ;(doc as any).autoTable({
    startY: 45,
    head: [['ID', 'Название', 'Юридическое название', 'Email', 'ИНН/БИН', 'Страна', 'Статус', 'Проектов']],
    body: tableData,
    theme: 'striped',
    headStyles: { fillColor: [59, 130, 246] },
    styles: { fontSize: 8 },
    margin: { top: 45 },
  })
  
  // Сохраняем PDF
  const fileName = `clients_${new Date().toISOString().split('T')[0]}.pdf`
  doc.save(fileName)
}

/**
 * Экспортирует клиентов в Word (DOCX) формат
 */
export async function exportClientsToWord(clients: ClientExportData[]): Promise<void> {
  if (clients.length === 0) {
    alert('Нет данных для экспорта')
    return
  }

  const children: (Paragraph | Table)[] = []
  
  // Заголовок
  children.push(
    new Paragraph({
      text: 'Список клиентов',
      heading: HeadingLevel.HEADING_1,
      alignment: AlignmentType.CENTER,
      spacing: { after: 200 },
    })
  )
  
  children.push(
    new Paragraph({
      children: [
        new TextRun({ text: 'Экспортировано: ', bold: true }),
        new TextRun({ text: new Date().toLocaleDateString('ru-RU') }),
      ],
      spacing: { after: 100 },
    })
  )
  
  children.push(
    new Paragraph({
      children: [
        new TextRun({ text: 'Всего клиентов: ', bold: true }),
        new TextRun({ text: clients.length.toString() }),
      ],
      spacing: { after: 300 },
    })
  )
  
  // Таблица клиентов
  const tableRows = [
    new TableRow({
      children: [
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'ID', bold: true })] })] }),
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Название', bold: true })] })] }),
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Юридическое название', bold: true })] })] }),
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Email', bold: true })] })] }),
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'ИНН/БИН', bold: true })] })] }),
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Страна', bold: true })] })] }),
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Статус', bold: true })] })] }),
        new TableCell({ children: [new Paragraph({ children: [new TextRun({ text: 'Проектов', bold: true })] })] }),
      ],
    }),
    ...clients.slice(0, 100).map(client =>
      new TableRow({
        children: [
          new TableCell({ children: [new Paragraph(client.id.toString())] }),
          new TableCell({ children: [new Paragraph(client.name || '')] }),
          new TableCell({ children: [new Paragraph(client.legal_name || '')] }),
          new TableCell({ children: [new Paragraph(client.contact_email || '')] }),
          new TableCell({ children: [new Paragraph(client.tax_id || '')] }),
          new TableCell({ children: [new Paragraph(client.country ? (getCountryByCode(client.country)?.name || client.country) : '')] }),
          new TableCell({ children: [new Paragraph(client.status || '')] }),
          new TableCell({ children: [new Paragraph((client.project_count || 0).toString())] }),
        ],
      })
    ),
  ]
  
  if (clients.length > 100) {
    children.push(
      new Paragraph({
        children: [new TextRun({ text: `Показано 100 из ${clients.length} клиентов`, italics: true })],
        spacing: { before: 100 },
      })
    )
  }
  
  const clientsTable = new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: tableRows,
  })
  
  children.push(clientsTable)
  
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
  const fileName = `clients_${new Date().toISOString().split('T')[0]}.docx`
  saveAs(blob, fileName)
}
