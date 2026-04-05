package db

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"anomaly_detection_system/backend/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dataDir string) {
	dbDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create db directory: %v", err)
	}

	dbPath := filepath.Join(dbDir, "app.db")
	primaryDB, err := openAndMigrate(dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := writeProbe(primaryDB); err == nil {
		DB = primaryDB
		log.Printf("Database initialized at %s", dbPath)
		return
	} else if !isReadonlyErr(err) {
		log.Fatalf("Database write probe failed: %v", err)
	}

	log.Printf("Database path is readonly (%s), falling back to writable local cache", dbPath)
	fallbackDir := filepath.Join(os.TempDir(), "anomaly_detection_system", "db")
	if err := os.MkdirAll(fallbackDir, 0755); err != nil {
		log.Fatalf("Failed to create fallback db directory: %v", err)
	}
	fallbackPath := filepath.Join(fallbackDir, "app.db")
	if err := copyFile(dbPath, fallbackPath); err != nil {
		log.Printf("Warning: failed to copy readonly DB to fallback path: %v", err)
	}

	closeGorm(primaryDB)
	fallbackDB, err := openAndMigrate(fallbackPath)
	if err != nil {
		log.Fatalf("Failed to connect to fallback database: %v", err)
	}
	if err := writeProbe(fallbackDB); err != nil {
		log.Fatalf("Fallback database is not writable: %v", err)
	}
	DB = fallbackDB
	log.Printf("Database initialized at fallback path %s", fallbackPath)
}

func openAndMigrate(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&models.Sample{}, &models.TrainingRun{}, &models.EvalRun{}, &models.RuntimeState{}); err != nil {
		return nil, err
	}

	if db.Migrator().HasIndex(&models.Sample{}, "idx_samples_frame_id") {
		if err := db.Migrator().DropIndex(&models.Sample{}, "idx_samples_frame_id"); err != nil {
			return nil, err
		}
	}

	if !db.Migrator().HasIndex(&models.Sample{}, "uniq_run_frame") {
		if err := db.Migrator().CreateIndex(&models.Sample{}, "uniq_run_frame"); err != nil {
			return nil, err
		}
	}

	return db, nil
}

func writeProbe(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS __write_probe (id INTEGER PRIMARY KEY, ts TEXT);`).Error; err != nil {
		return err
	}
	if err := db.Exec(`INSERT INTO __write_probe (ts) VALUES (datetime('now'));`).Error; err != nil {
		return err
	}
	if err := db.Exec(`DELETE FROM __write_probe WHERE id NOT IN (SELECT id FROM __write_probe ORDER BY id DESC LIMIT 1);`).Error; err != nil {
		return err
	}
	return nil
}

func isReadonlyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "readonly") || strings.Contains(msg, "read-only")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil {
			log.Printf("close fallback db file error: %v", cerr)
		}
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	return nil
}

func closeGorm(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("close db warning: %v", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Printf("close db warning: %v", err)
	}
}
