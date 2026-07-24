package service

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	userImportSheet        = "用户导入"
	fieldDescriptionSheet  = "字段说明"
	dataDictionarySheet    = "数据字典"
	roleValidationColumn   = "M"
	statusValidationColumn = "N"
	decoratedHeaderRow     = 4
	decoratedDataStartRow  = 5
	templateValidationRows = 1000
)

// parsedImportRow 保留数据值和原始 Excel 行号，避免新版模板的装饰行导致错误定位偏移。
type parsedImportRow struct {
	Number int
	Values []string
}

// workbookStyles 汇总同一工作簿内复用的样式编号，业务编排无需感知 excelize 样式细节。
type workbookStyles struct {
	Title       int
	Subtitle    int
	Header      int
	Body        int
	Example     int
	SectionNote int
}

// parseXLSX 使用 excelize 读取第一个数据工作表，兼容旧模板与隐藏系统字段行的新模板，并拒绝宏和公式单元格。
func parseXLSX(data []byte, maxRows int) ([]string, []parsedImportRow, error) {
	if err := rejectMacroWorkbook(data); err != nil {
		return nil, nil, err
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("打开 Excel 失败: %w", err)
	}
	defer workbook.Close()
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil, errors.New("Excel 没有可读取的工作表")
	}
	rows, err := workbook.GetRows(sheets[0])
	if err != nil {
		return nil, nil, errors.New("Excel 工作表解析失败")
	}
	if len(rows) < 2 {
		return nil, nil, errors.New("Excel 至少需要一行标题和一行数据")
	}
	if err := rejectFormulaCells(workbook, sheets[0], rows); err != nil {
		return nil, nil, err
	}
	decorated := isDecoratedImportTemplate(rows)
	headerWidth := len(rows[0])
	if decorated {
		headerWidth = len(userImportHeaders)
	}
	headers := normalizeRow(rows[0], headerWidth)
	dataStart := 1
	if decorated {
		dataStart = decoratedDataStartRow - 1
	}
	parsedRows := make([]parsedImportRow, 0, len(rows)-dataStart)
	for rowIndex := dataStart; rowIndex < len(rows); rowIndex++ {
		if len(parsedRows) >= maxRows {
			return nil, nil, fmt.Errorf("%w: Excel 数据行超过服务端限制 %d", ErrImportRowLimitExceeded, maxRows)
		}
		values := normalizeRow(rows[rowIndex], len(headers))
		if isBlankRow(values) {
			continue
		}
		parsedRows = append(parsedRows, parsedImportRow{Number: rowIndex + 1, Values: values})
	}
	return headers, parsedRows, nil
}

// rejectMacroWorkbook 在 excelize 解析前检查 VBA 项，防止把带宏文件伪装成普通 xlsx 上传。
func rejectMacroWorkbook(data []byte) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("打开 Excel 失败: %w", err)
	}
	for _, file := range reader.File {
		if strings.HasSuffix(strings.ToLower(file.Name), "vbaproject.bin") {
			return errors.New("不支持包含宏的 Excel 文件")
		}
	}
	return nil
}

// rejectFormulaCells 扫描已使用区域并拒绝公式，确保服务端只处理管理员明确提交的文本值。
func rejectFormulaCells(workbook *excelize.File, sheet string, rows [][]string) error {
	for rowIndex, row := range rows {
		for columnIndex := range row {
			cell, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
			if err != nil {
				return errors.New("Excel 单元格坐标无效")
			}
			formula, err := workbook.GetCellFormula(sheet, cell)
			if err != nil {
				return errors.New("Excel 公式检查失败")
			}
			if strings.TrimSpace(formula) != "" {
				return errors.New("不支持包含公式的导入单元格，请粘贴为纯文本值")
			}
		}
	}
	return nil
}

// isDecoratedImportTemplate 识别本系统生成的标题和中文表头装饰区，避免跳过普通旧版数据行。
func isDecoratedImportTemplate(rows [][]string) bool {
	if len(rows) < decoratedDataStartRow || len(rows[1]) == 0 || len(rows[3]) == 0 {
		return false
	}
	title := strings.TrimSpace(rows[1][0])
	firstDisplayHeader := strings.TrimSpace(rows[3][0])
	return strings.HasSuffix(title, "批量导入模板") && strings.HasPrefix(firstDisplayHeader, "用户名")
}

// normalizeRow 按系统字段数裁剪或补齐一行，并移除单元格两侧空白。
func normalizeRow(row []string, width int) []string {
	values := make([]string, width)
	for index := 0; index < width && index < len(row); index++ {
		values[index] = strings.TrimSpace(row[index])
	}
	return values
}

// isBlankRow 判断数据行是否全为空，避免把模板尾部预留输入区写入批次。
func isBlankRow(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

// buildXLSX 根据字段定义生成模板或错误报告；用户模板使用三工作表企业布局，其他输出使用标准表格布局。
func buildXLSX(headers []string, rows [][]string, instructions [][]string) ([]byte, error) {
	workbook := excelize.NewFile()
	defer workbook.Close()
	styles, err := createWorkbookStyles(workbook)
	if err != nil {
		return nil, err
	}
	if instructions != nil && sameHeaders(headers, userImportHeaders) {
		err = buildUserImportWorkbook(workbook, styles, headers, rows, instructions)
	} else {
		err = buildStandardWorkbook(workbook, styles, headers, rows, instructions)
	}
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := workbook.Write(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// buildUserImportWorkbook 创建用户数据页、字段说明页和数据字典页，并保持第一行系统 key 兼容旧字段映射。
func buildUserImportWorkbook(workbook *excelize.File, styles workbookStyles, headers []string, rows, instructions [][]string) error {
	if err := workbook.SetSheetName("Sheet1", userImportSheet); err != nil {
		return err
	}
	if _, err := workbook.NewSheet(fieldDescriptionSheet); err != nil {
		return err
	}
	if _, err := workbook.NewSheet(dataDictionarySheet); err != nil {
		return err
	}
	if err := writeUserDataSheet(workbook, styles, headers, rows); err != nil {
		return err
	}
	if err := writeFieldDescriptionSheet(workbook, styles, instructions); err != nil {
		return err
	}
	if err := writeDataDictionarySheet(workbook, styles); err != nil {
		return err
	}
	workbook.SetActiveSheet(0)
	return nil
}

// writeUserDataSheet 构建管理员实际填写的数据页，隐藏系统映射行并提供可见中文表头与录入辅助。
func writeUserDataSheet(workbook *excelize.File, styles workbookStyles, headers []string, rows [][]string) error {
	lastColumn, err := excelize.ColumnNumberToName(len(headers))
	if err != nil {
		return err
	}
	if err := setRowValues(workbook, userImportSheet, 1, headers); err != nil {
		return err
	}
	if err := workbook.SetRowVisible(userImportSheet, 1, false); err != nil {
		return err
	}
	if err := workbook.MergeCell(userImportSheet, "A2", lastColumn+"2"); err != nil {
		return err
	}
	if err := workbook.SetCellStr(userImportSheet, "A2", "用户批量导入模板"); err != nil {
		return err
	}
	if err := workbook.SetCellStyle(userImportSheet, "A2", lastColumn+"2", styles.Title); err != nil {
		return err
	}
	if err := workbook.MergeCell(userImportSheet, "A3", lastColumn+"3"); err != nil {
		return err
	}
	if err := workbook.SetCellStr(userImportSheet, "A3", "带 * 字段为必填，请按照示例格式填写；请删除或覆盖示例数据后再上传。"); err != nil {
		return err
	}
	if err := workbook.SetCellStyle(userImportSheet, "A3", lastColumn+"3", styles.Subtitle); err != nil {
		return err
	}
	if err := setRowValues(workbook, userImportSheet, decoratedHeaderRow, userDisplayHeaders(headers)); err != nil {
		return err
	}
	if err := workbook.SetCellStyle(userImportSheet, "A4", lastColumn+"4", styles.Header); err != nil {
		return err
	}
	for index, row := range rows {
		rowNumber := decoratedDataStartRow + index
		if err := setRowValues(workbook, userImportSheet, rowNumber, row); err != nil {
			return err
		}
		if err := workbook.SetCellStyle(userImportSheet, fmt.Sprintf("A%d", rowNumber), fmt.Sprintf("%s%d", lastColumn, rowNumber), styles.Example); err != nil {
			return err
		}
	}
	inputEndRow := decoratedDataStartRow + 20
	if err := workbook.SetCellStyle(userImportSheet, fmt.Sprintf("A%d", decoratedDataStartRow), fmt.Sprintf("%s%d", lastColumn, inputEndRow), styles.Body); err != nil {
		return err
	}
	for index := range rows {
		rowNumber := decoratedDataStartRow + index
		if err := workbook.SetCellStyle(userImportSheet, fmt.Sprintf("A%d", rowNumber), fmt.Sprintf("%s%d", lastColumn, rowNumber), styles.Example); err != nil {
			return err
		}
	}
	if err := setUserColumnWidths(workbook, userImportSheet); err != nil {
		return err
	}
	if err := workbook.SetRowHeight(userImportSheet, 2, 34); err != nil {
		return err
	}
	if err := workbook.SetRowHeight(userImportSheet, 3, 30); err != nil {
		return err
	}
	if err := workbook.SetRowHeight(userImportSheet, 4, 32); err != nil {
		return err
	}
	if err := workbook.SetPanes(userImportSheet, &excelize.Panes{Freeze: true, YSplit: 4, TopLeftCell: "A5", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	filterEndRow := decoratedDataStartRow + templateValidationRows - 1
	if err := workbook.AutoFilter(userImportSheet, fmt.Sprintf("A4:%s%d", lastColumn, filterEndRow), nil); err != nil {
		return err
	}
	if err := writeValidationSource(workbook, userImportSheet, roleValidationColumn, []string{"DO", "DU", "DO,DU", "TENANT_ADMIN"}); err != nil {
		return err
	}
	if err := writeValidationSource(workbook, userImportSheet, statusValidationColumn, []string{"ACTIVE", "DISABLED"}); err != nil {
		return err
	}
	if err := addValidation(workbook, userImportSheet, fmt.Sprintf("F5:F%d", filterEndRow), "$M$1:$M$4", "角色编码", "请选择角色组合，多角色使用英文逗号分隔"); err != nil {
		return err
	}
	return addValidation(workbook, userImportSheet, fmt.Sprintf("G5:G%d", filterEndRow), "$N$1:$N$2", "成员状态", "请选择启用或禁用状态")
}

// writeFieldDescriptionSheet 将系统字段、必填性、示例和规则整理为管理员可读的说明表。
func writeFieldDescriptionSheet(workbook *excelize.File, styles workbookStyles, instructions [][]string) error {
	if err := writeSectionTitle(workbook, fieldDescriptionSheet, "A1", "D1", "用户导入字段说明", styles.Title); err != nil {
		return err
	}
	if err := writeSectionNote(workbook, fieldDescriptionSheet, "A2", "D2", "系统字段仅用于映射，请在“用户导入”工作表按中文表头填写。", styles.SectionNote); err != nil {
		return err
	}
	headers := []string{"字段", "是否必填", "示例", "说明"}
	if err := setRowValues(workbook, fieldDescriptionSheet, 4, headers); err != nil {
		return err
	}
	if err := workbook.SetCellStyle(fieldDescriptionSheet, "A4", "D4", styles.Header); err != nil {
		return err
	}
	for index, item := range instructions {
		if len(item) < 5 {
			continue
		}
		row := []string{item[0], item[2], item[3], item[1] + "；" + item[4]}
		rowNumber := 5 + index
		if err := setRowValues(workbook, fieldDescriptionSheet, rowNumber, row); err != nil {
			return err
		}
		if err := workbook.SetCellStyle(fieldDescriptionSheet, fmt.Sprintf("A%d", rowNumber), fmt.Sprintf("D%d", rowNumber), styles.Body); err != nil {
			return err
		}
	}
	if err := setColumnWidths(workbook, fieldDescriptionSheet, map[string]float64{"A": 24, "B": 14, "C": 24, "D": 58}); err != nil {
		return err
	}
	if err := workbook.SetRowHeight(fieldDescriptionSheet, 1, 32); err != nil {
		return err
	}
	if err := workbook.SetRowHeight(fieldDescriptionSheet, 2, 28); err != nil {
		return err
	}
	return workbook.SetPanes(fieldDescriptionSheet, &excelize.Panes{Freeze: true, YSplit: 4, TopLeftCell: "A5", ActivePane: "bottomLeft"})
}

// writeDataDictionarySheet 提供角色和成员状态的稳定编码说明，降低管理员在数据页反复试错的成本。
func writeDataDictionarySheet(workbook *excelize.File, styles workbookStyles) error {
	if err := writeSectionTitle(workbook, dataDictionarySheet, "A1", "C1", "用户导入数据字典", styles.Title); err != nil {
		return err
	}
	if err := writeSectionNote(workbook, dataDictionarySheet, "A2", "C2", "下拉选项使用系统编码，中文说明仅用于帮助理解。", styles.SectionNote); err != nil {
		return err
	}
	if err := setRowValues(workbook, dataDictionarySheet, 4, []string{"字典类型", "编码", "说明"}); err != nil {
		return err
	}
	if err := workbook.SetCellStyle(dataDictionarySheet, "A4", "C4", styles.Header); err != nil {
		return err
	}
	rows := [][]string{{"角色编码", "DO", "数据拥有者"}, {"角色编码", "DU", "数据使用者"}, {"角色编码", "TENANT_ADMIN", "租户管理员"}, {"成员状态", "ACTIVE", "启用"}, {"成员状态", "DISABLED", "禁用"}}
	for index, row := range rows {
		rowNumber := 5 + index
		if err := setRowValues(workbook, dataDictionarySheet, rowNumber, row); err != nil {
			return err
		}
		if err := workbook.SetCellStyle(dataDictionarySheet, fmt.Sprintf("A%d", rowNumber), fmt.Sprintf("C%d", rowNumber), styles.Body); err != nil {
			return err
		}
	}
	if err := setColumnWidths(workbook, dataDictionarySheet, map[string]float64{"A": 18, "B": 22, "C": 36}); err != nil {
		return err
	}
	if err := workbook.SetRowHeight(dataDictionarySheet, 1, 32); err != nil {
		return err
	}
	return workbook.SetPanes(dataDictionarySheet, &excelize.Panes{Freeze: true, YSplit: 4, TopLeftCell: "A5", ActivePane: "bottomLeft"})
}

// buildStandardWorkbook 为组织模板和错误报告创建轻量专业表格，并维持第一行字段语义不变。
func buildStandardWorkbook(workbook *excelize.File, styles workbookStyles, headers []string, rows, instructions [][]string) error {
	if err := workbook.SetSheetName("Sheet1", "导入数据"); err != nil {
		return err
	}
	lastColumn, err := excelize.ColumnNumberToName(len(headers))
	if err != nil {
		return err
	}
	if err := setRowValues(workbook, "导入数据", 1, headers); err != nil {
		return err
	}
	if err := workbook.SetCellStyle("导入数据", "A1", lastColumn+"1", styles.Header); err != nil {
		return err
	}
	for index, row := range rows {
		rowNumber := index + 2
		if err := setRowValues(workbook, "导入数据", rowNumber, row); err != nil {
			return err
		}
		if err := workbook.SetCellStyle("导入数据", fmt.Sprintf("A%d", rowNumber), fmt.Sprintf("%s%d", lastColumn, rowNumber), styles.Body); err != nil {
			return err
		}
	}
	if err := workbook.SetPanes("导入数据", &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	filterEnd := len(rows) + 1
	if filterEnd < 2 {
		filterEnd = 2
	}
	if err := workbook.AutoFilter("导入数据", fmt.Sprintf("A1:%s%d", lastColumn, filterEnd), nil); err != nil {
		return err
	}
	for index := range headers {
		column, _ := excelize.ColumnNumberToName(index + 1)
		if err := workbook.SetColWidth("导入数据", column, column, 20); err != nil {
			return err
		}
	}
	if instructions == nil {
		return nil
	}
	if _, err := workbook.NewSheet(fieldDescriptionSheet); err != nil {
		return err
	}
	if err := setRowValues(workbook, fieldDescriptionSheet, 1, []string{"字段", "说明", "必填", "示例", "校验规则"}); err != nil {
		return err
	}
	if err := workbook.SetCellStyle(fieldDescriptionSheet, "A1", "E1", styles.Header); err != nil {
		return err
	}
	for index, row := range instructions {
		rowNumber := index + 2
		if err := setRowValues(workbook, fieldDescriptionSheet, rowNumber, row); err != nil {
			return err
		}
		if err := workbook.SetCellStyle(fieldDescriptionSheet, fmt.Sprintf("A%d", rowNumber), fmt.Sprintf("E%d", rowNumber), styles.Body); err != nil {
			return err
		}
	}
	return setColumnWidths(workbook, fieldDescriptionSheet, map[string]float64{"A": 24, "B": 42, "C": 12, "D": 24, "E": 44})
}

// createWorkbookStyles 统一创建标题、说明、表头、正文和示例样式，避免业务函数散落颜色与边框常量。
func createWorkbookStyles(workbook *excelize.File) (workbookStyles, error) {
	var styles workbookStyles
	var err error
	if styles.Title, err = createTitleStyle(workbook); err != nil {
		return styles, err
	}
	if styles.Subtitle, err = createSubtitleStyle(workbook); err != nil {
		return styles, err
	}
	if styles.Header, err = createHeaderStyle(workbook); err != nil {
		return styles, err
	}
	if styles.Body, err = createBodyStyle(workbook, "#FFFFFF"); err != nil {
		return styles, err
	}
	if styles.Example, err = createBodyStyle(workbook, "#F2F7FC"); err != nil {
		return styles, err
	}
	styles.SectionNote, err = createSectionNoteStyle(workbook)
	return styles, err
}

// createTitleStyle 创建深蓝标题样式，用明显但克制的层级标识正式 SaaS 模板。
func createTitleStyle(workbook *excelize.File) (int, error) {
	return workbook.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 18, Color: "#FFFFFF", Family: "Microsoft YaHei"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#173F5F"}}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}, Border: []excelize.Border{{Type: "bottom", Color: "#0F2F49", Style: 2}}})
}

// createSubtitleStyle 创建浅蓝提示带，承载必填规则而不抢占主标题视觉焦点。
func createSubtitleStyle(workbook *excelize.File) (int, error) {
	return workbook.NewStyle(&excelize.Style{Font: &excelize.Font{Size: 10, Color: "#36566F", Family: "Microsoft YaHei"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#EAF2F8"}}, Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true}, Border: []excelize.Border{{Type: "bottom", Color: "#B7C9D6", Style: 1}}})
}

// createHeaderStyle 创建蓝色白字表头，保证筛选图标、长字段名和必填标识清晰可读。
func createHeaderStyle(workbook *excelize.File) (int, error) {
	return workbook.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 10, Color: "#FFFFFF", Family: "Microsoft YaHei"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#2F75B5"}}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}, Border: []excelize.Border{{Type: "left", Color: "#D6E1EA", Style: 1}, {Type: "right", Color: "#D6E1EA", Style: 1}, {Type: "top", Color: "#245B8C", Style: 1}, {Type: "bottom", Color: "#245B8C", Style: 1}}})
}

// createBodyStyle 创建数据区轻边框样式；fillColor 用于区分示例行和管理员输入区。
func createBodyStyle(workbook *excelize.File, fillColor string) (int, error) {
	return workbook.NewStyle(&excelize.Style{Font: &excelize.Font{Size: 10, Color: "#253746", Family: "Microsoft YaHei"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{fillColor}}, Alignment: &excelize.Alignment{Vertical: "center", WrapText: true}, Border: []excelize.Border{{Type: "left", Color: "#D9E2E8", Style: 1}, {Type: "right", Color: "#D9E2E8", Style: 1}, {Type: "top", Color: "#D9E2E8", Style: 1}, {Type: "bottom", Color: "#D9E2E8", Style: 1}}})
}

// createSectionNoteStyle 创建说明页提示样式，强调系统字段不可修改这一兼容边界。
func createSectionNoteStyle(workbook *excelize.File) (int, error) {
	return workbook.NewStyle(&excelize.Style{Font: &excelize.Font{Size: 10, Color: "#7A4E00", Family: "Microsoft YaHei"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#FFF4CE"}}, Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true}, Border: []excelize.Border{{Type: "bottom", Color: "#E6C96C", Style: 1}}})
}

// setUserColumnWidths 为用户数据页设置语义化列宽，避免邮箱、角色和属性内容被截断。
func setUserColumnWidths(workbook *excelize.File, sheet string) error {
	return setColumnWidths(workbook, sheet, map[string]float64{"A": 18, "B": 18, "C": 30, "D": 18, "E": 16, "F": 20, "G": 15, "H": 18, "I": 18, "J": 30, "K": 20})
}

// setColumnWidths 统一应用列宽配置，任一列失败时立即返回以避免生成半成品模板。
func setColumnWidths(workbook *excelize.File, sheet string, widths map[string]float64) error {
	for column, width := range widths {
		if err := workbook.SetColWidth(sheet, column, column, width); err != nil {
			return err
		}
	}
	return nil
}

// writeValidationSource 将包含逗号的合法组合逐项写入隐藏辅助列，避免 Excel 把 DO,DU 错拆成两个选项。
func writeValidationSource(workbook *excelize.File, sheet, column string, values []string) error {
	for index, value := range values {
		cell := fmt.Sprintf("%s%d", column, index+1)
		if err := workbook.SetCellStr(sheet, cell, value); err != nil {
			return err
		}
	}
	return workbook.SetColVisible(sheet, column, false)
}

// addValidation 为可编辑列增加引用隐藏辅助区的阻止型下拉校验；服务端校验仍是最终安全边界。
func addValidation(workbook *excelize.File, sheet, cellRange, sourceRange, title, prompt string) error {
	validation := excelize.NewDataValidation(true)
	validation.SetSqref(cellRange)
	validation.SetSqrefDropList(sourceRange)
	validation.SetInput(title, prompt)
	validation.SetError(excelize.DataValidationErrorStyleStop, "输入值不在允许范围内", "请使用单元格下拉列表中的有效编码")
	return workbook.AddDataValidation(sheet, validation)
}

// writeSectionTitle 合并并写入说明类工作表标题，保持三张工作表一致的视觉语言。
func writeSectionTitle(workbook *excelize.File, sheet, startCell, endCell, title string, style int) error {
	if err := workbook.MergeCell(sheet, startCell, endCell); err != nil {
		return err
	}
	if err := workbook.SetCellStr(sheet, startCell, title); err != nil {
		return err
	}
	return workbook.SetCellStyle(sheet, startCell, endCell, style)
}

// writeSectionNote 合并并写入说明提示，使管理员在离开数据页后仍能看到关键兼容规则。
func writeSectionNote(workbook *excelize.File, sheet, startCell, endCell, note string, style int) error {
	if err := workbook.MergeCell(sheet, startCell, endCell); err != nil {
		return err
	}
	if err := workbook.SetCellStr(sheet, startCell, note); err != nil {
		return err
	}
	return workbook.SetCellStyle(sheet, startCell, endCell, style)
}

// setRowValues 按文本写入整行并统一执行公式注入转义，避免模板和错误报告产生可执行单元格。
func setRowValues(workbook *excelize.File, sheet string, rowNumber int, values []string) error {
	for columnIndex, value := range values {
		if value == "" {
			continue
		}
		cell, err := excelize.CoordinatesToCellName(columnIndex+1, rowNumber)
		if err != nil {
			return err
		}
		if err := workbook.SetCellStr(sheet, cell, sanitizeSpreadsheetValue(value)); err != nil {
			return err
		}
	}
	return nil
}

// userDisplayHeaders 将稳定系统 key 映射为面向租户管理员的中文字段名，不改变后端字段顺序。
func userDisplayHeaders(headers []string) []string {
	labels := map[string]string{"username": "用户名*", "display_name": "显示名称*", "email": "邮箱", "phone": "手机号", "org_code": "组织编码*", "role_codes": "角色编码*", "member_status": "成员状态", "job_title": "职位", "employee_no": "员工编号", "attributes": "用户属性", "initial_password": "初始密码"}
	display := make([]string, len(headers))
	for index, header := range headers {
		if label, ok := labels[header]; ok {
			display[index] = label
		} else {
			display[index] = header
		}
	}
	return display
}

// sameHeaders 精确比较字段顺序，只有用户导入定义才能启用专用三工作表布局。
func sameHeaders(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// sanitizeSpreadsheetValue 防止错误报告或模板中的文本被 Excel 解释为公式。
func sanitizeSpreadsheetValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" && strings.ContainsRune("=+-@", []rune(trimmed)[0]) {
		return "'" + value
	}
	return value
}
