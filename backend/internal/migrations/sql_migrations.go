package migrations

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// RunExplicitMigrations 执行本阶段需要的显式 SQL 迁移。
//
// 当前仓库早期 SQL 脚本并非全部幂等，直接从 001 重放会破坏已有开发库。因此这里先只接入
// 本阶段新增且声明幂等的 RBAC 迁移文件，后续若要全量迁移流水线，应先补齐历史脚本的迁移记录。
func RunExplicitMigrations(db *gorm.DB) error {
	for _, name := range []string{"010_tenant_rbac.sql"} {
		if err := runSQLMigrationFile(db, name); err != nil {
			return err
		}
	}
	return nil
}

// runSQLMigrationFile 读取并顺序执行单个迁移文件，保留出错语句名便于定位失败原因。
func runSQLMigrationFile(db *gorm.DB, name string) error {
	path, err := findMigrationFile(name)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取迁移文件 %s 失败: %w", path, err)
	}
	statements, err := splitSQLStatements(string(content))
	if err != nil {
		return fmt.Errorf("解析迁移文件 %s 失败: %w", path, err)
	}
	for i, statement := range statements {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("执行迁移 %s 第 %d 条语句失败: %w", name, i+1, err)
		}
	}
	return nil
}

// findMigrationFile 从常见工作目录向上查找 migrations 文件夹，兼容 go run ./cmd/migrate 和 IDE 启动。
func findMigrationFile(name string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "migrations", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未找到迁移文件 %s", name)
		}
		dir = parent
	}
}

// splitSQLStatements 按 MySQL 客户端 DELIMITER 语法拆分语句，保证存储过程体内的分号不会被误切。
func splitSQLStatements(sqlText string) ([]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(sqlText))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	delimiter := ";"
	statements := make([]string, 0)
	var builder strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "DELIMITER ") {
			next := strings.TrimSpace(trimmed[len("DELIMITER "):])
			if next == "" {
				return nil, fmt.Errorf("DELIMITER 不能为空")
			}
			delimiter = next
			continue
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
		if strings.HasSuffix(trimmed, delimiter) {
			statement := strings.TrimSpace(builder.String())
			statement = strings.TrimSuffix(statement, delimiter)
			statement = strings.TrimSpace(statement)
			if statement != "" {
				statements = append(statements, statement)
			}
			builder.Reset()
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if remaining := strings.TrimSpace(builder.String()); remaining != "" {
		statements = append(statements, remaining)
	}
	return statements, nil
}
