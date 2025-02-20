// server.go
package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	_ "github.com/go-sql-driver/mysql"
)

// Employee と WorkRecord の構造体定義
type Employee struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Job  string `json:"job"`
}

type WorkRecord struct {
	EmployeeID int    `json:"employee_id"`
	TargetDate string `json:"target_date"`
	TargetTime string `json:"target_time"`
	TargetType string `json:"target_type"`
}

var db *sql.DB

// 従業員情報を取得するエンドポイント
func employeesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query("SELECT id, name, job FROM employees")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query Error:", err)
		return
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var emp Employee
		if err := rows.Scan(&emp.ID, &emp.Name, &emp.Job); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Scan Error:", err)
			return
		}
		employees = append(employees, emp)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(employees)
}

// 打刻情報をデータベースに保存するエンドポイント
func saveWorkRecordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var record WorkRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error:", err)
		return
	}

	query := "INSERT INTO work_records (employee_id, target_date, target_time, target_type) VALUES (?, ?, ?, ?)"
	_, err := db.Exec(query, record.EmployeeID, record.TargetDate, record.TargetTime, record.TargetType)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Insert Error:", err)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Record saved successfully!"))
}

func main() {
	var err error
	// MySQL への接続設定: ユーザー名、パスワード、ホスト、ポート、データベース名を指定
	dsn := "your_username:your_password@tcp(127.0.0.1:3306)/attendance?parseTime=true&charset=utf8mb4"
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Database connection error:", err)
	}

	// 接続テスト
	if err = db.Ping(); err != nil {
		log.Fatal("Database ping error:", err)
	}

	// 静的ファイル (HTML, CSS, JS) を ./static フォルダから配信
	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/api/employees", employeesHandler)
	http.HandleFunc("/api/saveWorkRecord", saveWorkRecordHandler)

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
