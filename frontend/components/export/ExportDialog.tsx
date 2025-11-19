"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Badge } from "@/components/ui/badge";
import { Download, FileText, Table as TableIcon, FileSpreadsheet } from "lucide-react";

const EXPORT_FORMATS = [
  { value: "csv", label: "CSV", icon: TableIcon, description: "Табличный формат для Excel" },
  { value: "json", label: "JSON", icon: FileText, description: "Структурированный формат" },
  { value: "excel", label: "Excel", icon: FileSpreadsheet, description: "Нативный формат Excel" },
];

const COLUMN_OPTIONS = [
  { id: "source_name", label: "Исходное наименование" },
  { id: "normalized_name", label: "Нормализованное наименование" },
  { id: "stage2_item_type", label: "Тип объекта (товар/услуга)" },
  { id: "final_code", label: "Финальный код" },
  { id: "final_name", label: "Финальное наименование" },
  { id: "final_confidence", label: "Уверенность" },
  { id: "final_processing_method", label: "Метод обработки" },
  { id: "stage6_classifier_code", label: "Код классификатора" },
  { id: "stage7_ai_code", label: "AI код" },
  { id: "kpved_code", label: "Код КПВЭД" },
  { id: "kpved_name", label: "Наименование КПВЭД" },
  { id: "category", label: "Категория" },
  { id: "merged_count", label: "Количество дубликатов" },
];

interface ExportDialogProps {
  trigger?: React.ReactNode;
  onExport?: (format: string, columns: string[]) => Promise<void>;
}

export function ExportDialog({ trigger, onExport }: ExportDialogProps) {
  const [open, setOpen] = useState(false);
  const [format, setFormat] = useState("csv");
  const [selectedColumns, setSelectedColumns] = useState<string[]>(
    COLUMN_OPTIONS.slice(0, 7).map(col => col.id)
  );
  const [isExporting, setIsExporting] = useState(false);

  const toggleColumn = (columnId: string) => {
    setSelectedColumns(prev =>
      prev.includes(columnId)
        ? prev.filter(id => id !== columnId)
        : [...prev, columnId]
    );
  };

  const selectAllColumns = () => {
    setSelectedColumns(COLUMN_OPTIONS.map(col => col.id));
  };

  const deselectAllColumns = () => {
    setSelectedColumns([]);
  };

  const handleExport = async () => {
    setIsExporting(true);

    try {
      if (onExport) {
        await onExport(format, selectedColumns);
      } else {
        // Default export implementation
        const response = await fetch(`/api/export/data?format=${format}`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            columns: selectedColumns,
            filters: {},
          }),
        });

        if (!response.ok) {
          throw new Error('Export failed');
        }

        const result = await response.json();

        // Если backend вернул путь к файлу, можно скачать
        if (result.filename) {
          alert(`Файл создан: ${result.filename}`);
        }
      }

      setOpen(false);
    } catch (error) {
      console.error('Export error:', error);
      alert('Ошибка при экспорте данных');
    } finally {
      setIsExporting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger || (
          <Button variant="outline">
            <Download className="mr-2 h-4 w-4" />
            Экспорт данных
          </Button>
        )}
      </DialogTrigger>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Экспорт нормализованных данных</DialogTitle>
          <DialogDescription>
            Выберите формат и данные для экспорта
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-6">
          {/* Формат экспорта */}
          <div className="space-y-3">
            <Label>Формат экспорта</Label>
            <RadioGroup value={format} onValueChange={setFormat} className="grid grid-cols-3 gap-4">
              {EXPORT_FORMATS.map((formatOption) => {
                const Icon = formatOption.icon;
                return (
                  <div key={formatOption.value}>
                    <RadioGroupItem
                      value={formatOption.value}
                      id={formatOption.value}
                      className="peer sr-only"
                    />
                    <Label
                      htmlFor={formatOption.value}
                      className="flex flex-col items-center justify-between rounded-md border-2 border-muted bg-popover p-4 hover:bg-accent hover:text-accent-foreground peer-data-[state=checked]:border-primary cursor-pointer transition-colors"
                    >
                      <Icon className="mb-3 h-6 w-6" />
                      <span className="font-medium">{formatOption.label}</span>
                      <span className="text-xs text-muted-foreground mt-1 text-center">
                        {formatOption.description}
                      </span>
                    </Label>
                  </div>
                );
              })}
            </RadioGroup>
          </div>

          {/* Выбор колонок */}
          <div className="space-y-3">
            <div className="flex justify-between items-center">
              <Label>Колонки для экспорта</Label>
              <div className="flex gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={selectAllColumns}
                  className="h-8 text-xs"
                >
                  Выбрать все
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={deselectAllColumns}
                  className="h-8 text-xs"
                >
                  Снять все
                </Button>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-2 max-h-60 overflow-y-auto p-3 border rounded-md">
              {COLUMN_OPTIONS.map((column) => (
                <div key={column.id} className="flex items-center space-x-2">
                  <Checkbox
                    id={column.id}
                    checked={selectedColumns.includes(column.id)}
                    onCheckedChange={() => toggleColumn(column.id)}
                  />
                  <Label
                    htmlFor={column.id}
                    className="text-sm cursor-pointer flex-1"
                  >
                    {column.label}
                  </Label>
                </div>
              ))}
            </div>
            <div className="flex items-center justify-between text-sm text-muted-foreground">
              <span>Выбрано колонок:</span>
              <Badge variant="secondary">
                {selectedColumns.length} из {COLUMN_OPTIONS.length}
              </Badge>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)} disabled={isExporting}>
            Отмена
          </Button>
          <Button onClick={handleExport} disabled={selectedColumns.length === 0 || isExporting}>
            {isExporting ? (
              <>
                <Download className="mr-2 h-4 w-4 animate-pulse" />
                Экспортируется...
              </>
            ) : (
              <>
                <Download className="mr-2 h-4 w-4" />
                Экспортировать
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
