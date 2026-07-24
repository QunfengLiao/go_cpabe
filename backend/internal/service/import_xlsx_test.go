package service

import (
	"bytes"
	"errors"
	"testing"

	"go-cpabe/backend/internal/domain"

	"github.com/xuri/excelize/v2"
)

// TestBuildAndParseImportTemplate 验证企业用户模板能被同一解析器读取，字段 key 和真实 Excel 行号保持不变。
func TestBuildAndParseImportTemplate(t *testing.T) {
	headers, rows, instructions, _, err := templateDefinition(domain.ImportTypeUsers)
	if err != nil {
		t.Fatalf("template definition: %v", err)
	}
	data, err := buildXLSX(headers, rows, instructions)
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	parsedHeaders, parsedRows, err := parseXLSX(data, 100)
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if len(parsedHeaders) != len(headers) || parsedHeaders[0] != "username" || parsedHeaders[5] != "role_codes" {
		t.Fatalf("unexpected headers: %#v", parsedHeaders)
	}
	if len(parsedRows) != len(rows) || parsedRows[0].Number != 5 || parsedRows[1].Values[5] != "DO,DU" {
		t.Fatalf("unexpected rows: %#v", parsedRows)
	}
}

// TestParseImportTemplateRowLimit 验证超过配置行数时返回可稳定映射的独立错误，而不是模糊文件错误。
func TestParseImportTemplateRowLimit(t *testing.T) {
	headers, rows, instructions, _, err := templateDefinition(domain.ImportTypeUsers)
	if err != nil {
		t.Fatalf("template definition: %v", err)
	}
	data, err := buildXLSX(headers, rows, instructions)
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	if _, _, err := parseXLSX(data, 1); !errors.Is(err, ErrImportRowLimitExceeded) {
		t.Fatalf("expected row limit error, got %v", err)
	}
}

// TestUserImportTemplateWorkbookDesign 验证三工作表、隐藏系统字段、中文表头、样式、冻结窗格和下拉校验均写入成品文件。
func TestUserImportTemplateWorkbookDesign(t *testing.T) {
	headers, rows, instructions, _, err := templateDefinition(domain.ImportTypeUsers)
	if err != nil {
		t.Fatalf("template definition: %v", err)
	}
	data, err := buildXLSX(headers, rows, instructions)
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	defer workbook.Close()
	sheets := workbook.GetSheetList()
	if len(sheets) != 3 || sheets[0] != userImportSheet || sheets[1] != fieldDescriptionSheet || sheets[2] != dataDictionarySheet {
		t.Fatalf("unexpected sheets: %#v", sheets)
	}
	visible, err := workbook.GetRowVisible(userImportSheet, 1)
	if err != nil || visible {
		t.Fatalf("system key row should be hidden: visible=%v err=%v", visible, err)
	}
	if title, _ := workbook.GetCellValue(userImportSheet, "A2"); title != "用户批量导入模板" {
		t.Fatalf("unexpected title: %q", title)
	}
	if displayHeader, _ := workbook.GetCellValue(userImportSheet, "F4"); displayHeader != "角色编码*" {
		t.Fatalf("unexpected display header: %q", displayHeader)
	}
	if style, _ := workbook.GetCellStyle(userImportSheet, "A4"); style == 0 {
		t.Fatal("header style should not use the default style")
	}
	panes, err := workbook.GetPanes(userImportSheet)
	if err != nil || !panes.Freeze || panes.YSplit != 4 {
		t.Fatalf("unexpected panes: %#v err=%v", panes, err)
	}
	validations, err := workbook.GetDataValidations(userImportSheet)
	if err != nil || len(validations) != 2 {
		t.Fatalf("unexpected validations: count=%d err=%v", len(validations), err)
	}
	if validations[0].Formula1 != "$M$1:$M$4" || validations[1].Formula1 != "$N$1:$N$2" {
		t.Fatalf("unexpected validation sources: %#v", validations)
	}
	if roleOption, _ := workbook.GetCellValue(userImportSheet, "M3"); roleOption != "DO,DU" {
		t.Fatalf("role combination must remain one dropdown option: %q", roleOption)
	}
	if visible, err := workbook.GetColVisible(userImportSheet, roleValidationColumn); err != nil || visible {
		t.Fatalf("validation source column should be hidden: visible=%v err=%v", visible, err)
	}
	if field, _ := workbook.GetCellValue(fieldDescriptionSheet, "A5"); field != "username" {
		t.Fatalf("unexpected field description: %q", field)
	}
	if dictionaryValue, _ := workbook.GetCellValue(dataDictionarySheet, "B7"); dictionaryValue != "TENANT_ADMIN" {
		t.Fatalf("unexpected dictionary value: %q", dictionaryValue)
	}
}

// TestParseLegacyImportTemplate 验证旧版第一行字段、第二行数据的工作簿仍可导入且行号不变。
func TestParseLegacyImportTemplate(t *testing.T) {
	workbook := excelize.NewFile()
	defer workbook.Close()
	if err := setRowValues(workbook, "Sheet1", 1, userImportHeaders); err != nil {
		t.Fatalf("write headers: %v", err)
	}
	legacyRow := []string{"legacy.user", "旧模板用户", "", "", "ROOT", "DU", "ACTIVE", "", "", "", ""}
	if err := setRowValues(workbook, "Sheet1", 2, legacyRow); err != nil {
		t.Fatalf("write row: %v", err)
	}
	var output bytes.Buffer
	if err := workbook.Write(&output); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	headers, rows, err := parseXLSX(output.Bytes(), 100)
	if err != nil {
		t.Fatalf("parse legacy xlsx: %v", err)
	}
	if headers[0] != "username" || len(rows) != 1 || rows[0].Number != 2 || rows[0].Values[0] != "legacy.user" {
		t.Fatalf("unexpected legacy parse result: headers=%#v rows=%#v", headers, rows)
	}
}

// TestValidateOrgCycles 验证导入组织关系出现循环时每个相关行都能收到可定位错误。
func TestValidateOrgCycles(t *testing.T) {
	rows := []domain.ImportRowResult{
		{RowNumber: 2, Key: "A", Status: domain.ImportRowValid, Fields: map[string]string{"parent_org_code": "B"}},
		{RowNumber: 3, Key: "B", Status: domain.ImportRowValid, Fields: map[string]string{"parent_org_code": "A"}},
	}
	validateOrgCycles(rows)
	for _, row := range rows {
		if row.Status != domain.ImportRowInvalid || len(row.Errors) == 0 || row.Errors[0].Code != "CYCLE" {
			t.Fatalf("row %s did not receive cycle error: %#v", row.Key, row)
		}
	}
}

// TestSpreadsheetFormulaSanitization 验证错误报告会转义可能被 Excel 当作公式的内容。
func TestSpreadsheetFormulaSanitization(t *testing.T) {
	for _, value := range []string{"=SUM(A1)", "+payload", "-payload", "@payload"} {
		if got := sanitizeSpreadsheetValue(value); got != "'"+value {
			t.Fatalf("formula value %q was not sanitized: %q", value, got)
		}
	}
	if got := sanitizeSpreadsheetValue("normal"); got != "normal" {
		t.Fatalf("normal value changed: %q", got)
	}
}

// TestImportHeaderValidation 验证必要字段标题不能被删除、改名或调换顺序。
func TestImportHeaderValidation(t *testing.T) {
	if err := validateHeaders(domain.ImportTypeUsers, userImportHeaders); err != nil {
		t.Fatalf("expected valid headers: %v", err)
	}
	bad := append([]string(nil), userImportHeaders...)
	bad[0] = "user_name"
	if err := validateHeaders(domain.ImportTypeUsers, bad); err == nil {
		t.Fatal("expected invalid header error")
	}
}

// TestImportPhoneValidation 验证手机号字段只允许常见号码字符和合理长度。
func TestImportPhoneValidation(t *testing.T) {
	if !validImportPhone("13800000000") {
		t.Fatal("expected valid phone")
	}
	if validImportPhone("phone<script>") {
		t.Fatal("expected invalid phone")
	}
}

// TestPublicImportRowsRedactsPasswordHash 验证确认事务所需的密码摘要不会随预览响应返回。
func TestPublicImportRowsRedactsPasswordHash(t *testing.T) {
	rows := publicImportRows([]domain.ImportRowResult{{Fields: map[string]string{"username": "alice", "initial_password_hash": "$2a$..."}}})
	if _, ok := rows[0].Fields["initial_password_hash"]; ok {
		t.Fatal("password hash must not be exposed in preview rows")
	}
}
