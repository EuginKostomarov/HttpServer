"use client"

import { useState, useEffect } from "react"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Input } from "@/components/ui/input"
import { Loader2, Play } from "lucide-react"

interface QualityAnalysisDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  selectedDatabase: string
  onStartAnalysis: (params: AnalysisParams) => Promise<void>
  analyzing: boolean
}

export interface AnalysisParams {
  table: string
  codeColumn: string
  nameColumn: string
}

export function QualityAnalysisDialog({
  open,
  onOpenChange,
  selectedDatabase,
  onStartAnalysis,
  analyzing
}: QualityAnalysisDialogProps) {
  const [analyzeTable, setAnalyzeTable] = useState<string>('normalized_data')
  const [analyzeCodeColumn, setAnalyzeCodeColumn] = useState<string>('')
  const [analyzeNameColumn, setAnalyzeNameColumn] = useState<string>('')

  // Reset defaults when table changes or dialog opens
  useEffect(() => {
    if (open) {
        handleTableChange(analyzeTable)
    }
  }, [open])

  const handleTableChange = (table: string) => {
    setAnalyzeTable(table)
    // Auto-fill columns based on table
    switch (table) {
      case 'normalized_data':
        setAnalyzeCodeColumn('code')
        setAnalyzeNameColumn('normalized_name')
        break
      case 'nomenclature_items':
        setAnalyzeCodeColumn('nomenclature_code')
        setAnalyzeNameColumn('nomenclature_name')
        break
      case 'catalog_items':
        setAnalyzeCodeColumn('code')
        setAnalyzeNameColumn('name')
        break
      default:
        setAnalyzeCodeColumn('')
        setAnalyzeNameColumn('')
    }
  }

  const handleSubmit = () => {
    // Determine default columns if empty
    let codeColumn = analyzeCodeColumn
    let nameColumn = analyzeNameColumn

    if (!codeColumn) {
      switch (analyzeTable) {
        case 'normalized_data': codeColumn = 'code'; break;
        case 'nomenclature_items': codeColumn = 'nomenclature_code'; break;
        case 'catalog_items': codeColumn = 'code'; break;
        default: codeColumn = 'code';
      }
    }

    if (!nameColumn) {
      switch (analyzeTable) {
        case 'normalized_data': nameColumn = 'normalized_name'; break;
        case 'nomenclature_items': nameColumn = 'nomenclature_name'; break;
        case 'catalog_items': nameColumn = 'name'; break;
        default: nameColumn = 'name';
      }
    }

    onStartAnalysis({
      table: analyzeTable,
      codeColumn,
      nameColumn
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>Запуск анализа качества</DialogTitle>
          <DialogDescription>
            Выберите таблицу для анализа качества данных. Анализ найдет дубликаты, нарушения правил и сгенерирует предложения по улучшению.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="table">Таблица</Label>
            <Select value={analyzeTable} onValueChange={handleTableChange}>
              <SelectTrigger id="table">
                <SelectValue placeholder="Выберите таблицу" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="normalized_data">normalized_data</SelectItem>
                <SelectItem value="nomenclature_items">nomenclature_items</SelectItem>
                <SelectItem value="catalog_items">catalog_items</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
                <Label htmlFor="code-column">Колонка с кодом</Label>
                <Input
                id="code-column"
                value={analyzeCodeColumn}
                onChange={(e) => setAnalyzeCodeColumn(e.target.value)}
                placeholder="Автозаполнение"
                />
            </div>
            <div className="space-y-2">
                <Label htmlFor="name-column">Колонка с названием</Label>
                <Input
                id="name-column"
                value={analyzeNameColumn}
                onChange={(e) => setAnalyzeNameColumn(e.target.value)}
                placeholder="Автозаполнение"
                />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={analyzing}>
            Отмена
          </Button>
          <Button onClick={handleSubmit} disabled={!selectedDatabase || analyzing}>
            {analyzing ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Запуск...
              </>
            ) : (
              <>
                <Play className="mr-2 h-4 w-4" />
                Запустить
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

