package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// JobType 構造体
type JobType struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Employee構造体
type Employee struct {
	ID                    int    `json:"id"`
	EmployeeNumber        string `json:"employee_number"`
	Name                  string `json:"name"`
	Job                   string `json:"job"`
	JobCode               string `json:"job_code"`
	MaxAttendanceCount    int    `json:"max_attendance_count"`
	PaidVacationLimit     int    `json:"paid_vacation_limit"`
	PaidVacationGrantDate string `json:"paid_vacation_grant_date"`
}

// WorkRecord構造体
type WorkRecord struct {
	EmployeeID int    `json:"employee_id"`
	TargetDate string `json:"target_date"`
	TargetTime string `json:"target_time"`
	TargetType string `json:"target_type"`
	Memo       string `json:"memo"`
}

var db *sql.DB

// MySQL接続
func initDB() {
	var err error
	dsn := "root:@tcp(127.0.0.1:3306)/attendance?parseTime=true&charset=utf8mb4"
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Database connection error:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Database ping error:", err)
	}
	fmt.Println("Database connected successfully!")
}

// 従業員情報を取得するAPI
func employeesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query(`
        SELECT e.employee_number, e.name, e.job, e.job_code, e.max_attendance_count, 
               e.paid_vacation_limit, e.paid_vacation_grant_date 
        FROM employees e
        JOIN job_types j ON e.job_code = j.code
    `)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query Error:", err)
		return
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var emp Employee
		if err := rows.Scan(&emp.EmployeeNumber, &emp.Name, &emp.Job, &emp.JobCode, &emp.MaxAttendanceCount, &emp.PaidVacationLimit, &emp.PaidVacationGrantDate); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Scan Error:", err)
			return
		}
		employees = append(employees, emp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(employees)
}

// 打刻情報を保存するAPI
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

// 従業員追加API
func addEmployeeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var emp Employee
	if err := json.NewDecoder(r.Body).Decode(&emp); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error:", err)
		return
	}

	// 職種名から職種コードを取得
	var jobCode string
	err := db.QueryRow("SELECT code FROM job_types WHERE name = ?", emp.Job).Scan(&jobCode)
	if err == sql.ErrNoRows {
		http.Error(w, "Invalid job name", http.StatusBadRequest)
		log.Println("Invalid job name:", emp.Job)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Database error:", err)
		return
	}

	// 従業員データを追加 (IDは自動生成)
	query := "INSERT INTO employees (name, job, job_code, max_attendance_count, paid_vacation_limit, paid_vacation_grant_date) VALUES (?, ?, ?, ?, ?, ?)"
	result, err := db.Exec(query, emp.Name, emp.Job, jobCode, emp.MaxAttendanceCount, emp.PaidVacationLimit, emp.PaidVacationGrantDate)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Insert Error:", err)
		return
	}

	// 挿入した従業員の ID を取得
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		http.Error(w, "Failed to get last insert ID", http.StatusInternalServerError)
		log.Println("Failed to get last insert ID:", err)
		return
	}

	// 従業員番号を作成（職種コード + 6桁ゼロ埋めID）
	employeeNumber := fmt.Sprintf("%s%06d", jobCode, lastInsertID)

	// 従業員番号を更新
	updateQuery := "UPDATE employees SET employee_number = ? WHERE id = ?"
	_, err = db.Exec(updateQuery, employeeNumber, lastInsertID)
	if err != nil {
		http.Error(w, "Failed to update employee number", http.StatusInternalServerError)
		log.Println("Update Error:", err)
		return
	}

	// 新しく追加された従業員の情報を JSON で返す
	newEmployee := Employee{
		EmployeeNumber:        employeeNumber,
		Name:                  emp.Name,
		Job:                   emp.Job,
		JobCode:               jobCode,
		MaxAttendanceCount:    emp.MaxAttendanceCount,
		PaidVacationLimit:     emp.PaidVacationLimit,
		PaidVacationGrantDate: emp.PaidVacationGrantDate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newEmployee)
}

// 職種一覧を取得する API
func jobTypesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query("SELECT code, name FROM job_types")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query Error:", err)
		return
	}
	defer rows.Close()

	var jobTypes []JobType
	for rows.Next() {
		var job JobType
		if err := rows.Scan(&job.Code, &job.Name); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Scan Error:", err)
			return
		}
		jobTypes = append(jobTypes, job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobTypes)
}

// 職種を追加する API
func addJobTypeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var job JobType
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error:", err)
		return
	}

	query := "INSERT INTO job_types (code, name) VALUES (?, ?)"
	_, err := db.Exec(query, job.Code, job.Name)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Insert Error:", err)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Job Type added successfully!"))
}

// 従業員削除 API
func deleteEmployeeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		EmployeeNumber string `json:"employee_number"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error:", err)
		return
	}

	// データベースから該当の従業員を削除
	query := "DELETE FROM employees WHERE employee_number = ?"
	result, err := db.Exec(query, request.EmployeeNumber)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Delete Error:", err)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		http.Error(w, "Employee not found or already deleted", http.StatusNotFound)
		log.Println("No employee found with number:", request.EmployeeNumber)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("従業員を削除しました"))
}

// 職種削除 API
func deleteJobTypeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Code string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error:", err)
		return
	}

	// 職種が従業員に関連付けられているか確認
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM employees WHERE job_code = ?", request.Code).Scan(&count)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Check Error:", err)
		return
	}

	if count > 0 {
		http.Error(w, "この職種は従業員に関連付けられています。削除できません。", http.StatusBadRequest)
		log.Println("Cannot delete job type:", request.Code, "as it is in use.")
		return
	}

	// データベースから該当の職種を削除
	query := "DELETE FROM job_types WHERE code = ?"
	result, err := db.Exec(query, request.Code)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Delete Error:", err)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		http.Error(w, "Job type not found or already deleted", http.StatusNotFound)
		log.Println("No job type found with code:", request.Code)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("職種を削除しました"))
}

// オーナー情報 (メールアドレス、パスワード) の受け取り構造体
type OwnerInfo struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// オーナー情報を取得するAPI
func getOwnerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var email, password sql.NullString
	err := db.QueryRow("SELECT owner_email, owner_password FROM system_config WHERE id = 1").
		Scan(&email, &password)
	if err != nil {
		if err == sql.ErrNoRows {
			// まだ設定されていない場合は空を返す
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(OwnerInfo{"", ""})
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query Error:", err)
		return
	}

	owner := OwnerInfo{
		Email:    email.String,
		Password: password.String, // 実運用ではパスワードを返さない方が安全
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(owner)
}

// オーナー情報を設定するAPI
func setOwnerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var info OwnerInfo
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error:", err)
		return
	}

	// ※パスワードをハッシュ化したい場合はここで処理
	// hashedPassword := hashFunc(info.Password)

	// system_config テーブルの id=1 を更新
	query := `
		UPDATE system_config
		SET owner_email = ?, owner_password = ?
		WHERE id = 1
	`
	_, err := db.Exec(query, info.Email, info.Password)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Update Error:", err)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("メールアドレスとパスワードを設定しました"))
}

// ログイン用のリクエストボディ
type LoginRequest struct {
	Password string `json:"password"`
}

// ログインAPI: パスワードチェック
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error in /api/login:", err)
		return
	}

	// system_config からパスワードを取得
	var storedPassword sql.NullString
	err := db.QueryRow("SELECT owner_password FROM system_config WHERE id = 1").Scan(&storedPassword)
	if err == sql.ErrNoRows {
		http.Error(w, "No password set", http.StatusUnauthorized)
		log.Println("No password found in system_config")
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query Error in /api/login:", err)
		return
	}

	// 照合
	if storedPassword.Valid && storedPassword.String == req.Password {
		// パスワード一致 → 200 OK
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Login success"))
	} else {
		// 不一致 → 401
		http.Error(w, "Invalid password", http.StatusUnauthorized)
	}
}

// employeeDetailHandler は、従業員番号をもとに該当従業員の詳細情報を取得してJSONで返します
func employeeDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	empNumber := r.URL.Query().Get("empNumber")
	if empNumber == "" {
		http.Error(w, "Missing employee number", http.StatusBadRequest)
		return
	}
	var emp Employee
	err := db.QueryRow(`
      SELECT id, employee_number, name, job, job_code, max_attendance_count, paid_vacation_limit, paid_vacation_grant_date 
      FROM employees WHERE employee_number = ?`, empNumber).
		Scan(&emp.ID, &emp.EmployeeNumber, &emp.Name, &emp.Job, &emp.JobCode, &emp.MaxAttendanceCount, &emp.PaidVacationLimit, &emp.PaidVacationGrantDate)
	if err != nil {
		http.Error(w, "Employee not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(emp)
}

// timeRecordsHandler は、従業員番号をもとに該当従業員のタイムレコーダー履歴を取得してJSONで返します
func timeRecordsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	empNumber := r.URL.Query().Get("empNumber")
	if empNumber == "" {
		http.Error(w, "Missing employee number", http.StatusBadRequest)
		return
	}

	// 従業員の内部IDを取得
	var empID int
	err := db.QueryRow("SELECT id FROM employees WHERE employee_number = ?", empNumber).Scan(&empID)
	if err != nil {
		http.Error(w, "Employee not found", http.StatusNotFound)
		return
	}

	// 月パラメータを取得（例："2025-02"）
	month := r.URL.Query().Get("month")
	var rows *sql.Rows
	if month != "" {
		// target_dateが"YYYY-MM-DD"形式の場合、LIKE句で月を指定（例："2025-02%"）
		rows, err = db.Query("SELECT employee_id, target_date, target_time, target_type, memo FROM work_records WHERE employee_id = ? AND target_date LIKE ? ORDER BY target_date ASC, target_time ASC", empID, month+"%")
	} else {
		rows, err = db.Query("SELECT employee_id, target_date, target_time, target_type, memo FROM work_records WHERE employee_id = ? ORDER BY target_date ASC, target_time ASC", empID)
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query Error in timeRecordsHandler:", err)
		return
	}
	defer rows.Close()

	var records []WorkRecord
	for rows.Next() {
		var rec WorkRecord
		if err := rows.Scan(&rec.EmployeeID, &rec.TargetDate, &rec.TargetTime, &rec.TargetType, &rec.Memo); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Scan Error in timeRecordsHandler:", err)
			return
		}
		records = append(records, rec)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// saveMemoHandler は、タイムレコーダーの各記録に対するメモを更新します。
func saveMemoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		EmployeeID int    `json:"employee_id"`
		TargetDate string `json:"target_date"`
		TargetTime string `json:"target_time"`
		Memo       string `json:"memo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error in /api/saveMemo:", err)
		return
	}
	// 更新クエリ（該当レコードが存在しない場合は影響を与えません）
	updateQuery := "UPDATE work_records SET memo = ? WHERE employee_id = ? AND target_date = ? AND target_time = ?"
	_, err := db.Exec(updateQuery, req.Memo, req.EmployeeID, req.TargetDate, req.TargetTime)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Update Error in /api/saveMemo:", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("メモを保存しました"))
}

// deleteTimeRecordHandler は、指定された employee_id, target_date, target_time, target_type のレコードを削除します
func deleteTimeRecordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		EmployeeID int    `json:"employee_id"`
		TargetDate string `json:"target_date"`
		TargetTime string `json:"target_time"`
		TargetType string `json:"target_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error in /api/deleteTimeRecord:", err)
		return
	}

	query := "DELETE FROM work_records WHERE employee_id = ? AND target_date = ? AND target_time = ? AND target_type = ?"
	result, err := db.Exec(query, req.EmployeeID, req.TargetDate, req.TargetTime, req.TargetType)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Delete Error in /api/deleteTimeRecord:", err)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		http.Error(w, "Time record not found or already deleted", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("タイムレコーダー記録を削除しました"))
}

// Inconsistency 不整合情報の構造体
type Inconsistency struct {
	EmployeeID     int      `json:"employee_id"`
	EmployeeNumber string   `json:"employee_number"`
	EmployeeName   string   `json:"employee_name"`
	Date           string   `json:"date"`
	Issues         []string `json:"issues"`
}

// checkInconsistencies は、同一日のwork_records群から不整合をチェックする関数
func checkInconsistencies(records []WorkRecord) []string {
	issues := []string{}

	var clockIns, clockOuts, breakStarts, breakEnds []WorkRecord
	for _, rec := range records {
		switch rec.TargetType {
		case "clock_in":
			clockIns = append(clockIns, rec)
		case "clock_out":
			clockOuts = append(clockOuts, rec)
		case "break_start":
			breakStarts = append(breakStarts, rec)
		case "break_end":
			breakEnds = append(breakEnds, rec)
		}
	}
	// 出勤のみ：出勤記録あり、退勤記録なし
	if len(clockIns) > 0 && len(clockOuts) == 0 {
		issues = append(issues, "出勤のみ（退勤記録なし）")
	}
	// 退勤のみ：退勤記録あり、出勤記録なし
	if len(clockOuts) > 0 && len(clockIns) == 0 {
		issues = append(issues, "退勤のみ（出勤記録なし）")
	}
	// 複数の出勤記録
	if len(clockIns) > 1 {
		issues = append(issues, "複数の出勤記録")
	}
	// 複数の退勤記録
	if len(clockOuts) > 1 {
		issues = append(issues, "複数の退勤記録")
	}
	// 休憩関連の不整合
	if len(breakStarts) != len(breakEnds) {
		issues = append(issues, "休憩記録不整合（開始と終了の数が一致しない）")
	}
	// 出勤・退勤の順序の不整合：最低の出勤時刻 > 最高の退勤時刻
	if len(clockIns) > 0 && len(clockOuts) > 0 {
		minClockIn := clockIns[0].TargetTime
		for _, rec := range clockIns {
			if rec.TargetTime < minClockIn {
				minClockIn = rec.TargetTime
			}
		}
		maxClockOut := clockOuts[0].TargetTime
		for _, rec := range clockOuts {
			if rec.TargetTime > maxClockOut {
				maxClockOut = rec.TargetTime
			}
		}
		if minClockIn > maxClockOut {
			issues = append(issues, "退勤記録が出勤記録より前")
		}
	}
	// 連続した同種の打刻（シンプルに先頭から順に同じタイプが連続していれば）
	for i := 1; i < len(records); i++ {
		if records[i].TargetType == records[i-1].TargetType {
			issues = append(issues, "連続した同種の打刻")
			break
		}
	}
	// 出勤前の休憩記録：出勤記録の最小時刻より前の休憩記録
	if len(clockIns) > 0 {
		minClockIn := clockIns[0].TargetTime
		for _, rec := range clockIns {
			if rec.TargetTime < minClockIn {
				minClockIn = rec.TargetTime
			}
		}
		for _, rec := range append(breakStarts, breakEnds...) {
			if rec.TargetTime < minClockIn {
				issues = append(issues, "出勤前の休憩記録")
				break
			}
		}
	}
	// 退勤後の休憩記録：退勤記録の最大時刻より後の休憩記録
	if len(clockOuts) > 0 {
		maxClockOut := clockOuts[0].TargetTime
		for _, rec := range clockOuts {
			if rec.TargetTime > maxClockOut {
				maxClockOut = rec.TargetTime
			}
		}
		for _, rec := range append(breakStarts, breakEnds...) {
			if rec.TargetTime > maxClockOut {
				issues = append(issues, "退勤後の休憩記録")
				break
			}
		}
	}
	return issues
}

// inconsistenciesHandler は、昨日までの不整合な打刻データをチェックして返すAPI
func inconsistenciesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 今日の日付より前（昨日まで）のデータを対象とする
	today := time.Now()
	cutoff := today.Format("2006-01-02") // 本日の日付文字列

	// target_dateは日付のみの形式で保存されていると仮定（YYYY-MM-DD）
	rows, err := db.Query("SELECT employee_id, target_date, target_time, target_type, memo FROM work_records WHERE target_date < ? ORDER BY target_date ASC, target_time ASC", cutoff)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query Error in inconsistenciesHandler:", err)
		return
	}
	defer rows.Close()

	// 従業員毎・日毎にグループ化
	groups := make(map[string][]WorkRecord)
	for rows.Next() {
		var rec WorkRecord
		if err := rows.Scan(&rec.EmployeeID, &rec.TargetDate, &rec.TargetTime, &rec.TargetType, &rec.Memo); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Scan Error in inconsistenciesHandler:", err)
			return
		}
		key := fmt.Sprintf("%d_%s", rec.EmployeeID, rec.TargetDate)
		groups[key] = append(groups[key], rec)
	}

	inconsistencies := []Inconsistency{}
	for key, recs := range groups {
		var empID int
		var date string
		fmt.Sscanf(key, "%d_%s", &empID, &date)
		issues := checkInconsistencies(recs)
		if len(issues) > 0 {
			// 従業員情報取得
			var emp Employee
			err := db.QueryRow("SELECT employee_number, name FROM employees WHERE id = ?", empID).Scan(&emp.EmployeeNumber, &emp.Name)
			if err != nil {
				continue
			}
			inconsistency := Inconsistency{
				EmployeeID:     empID,
				EmployeeNumber: emp.EmployeeNumber,
				EmployeeName:   emp.Name,
				Date:           date,
				Issues:         issues,
			}
			inconsistencies = append(inconsistencies, inconsistency)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(inconsistencies)
}

func main() {
	initDB()

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/api/employees", employeesHandler)
	http.HandleFunc("/api/addEmployee", addEmployeeHandler)
	http.HandleFunc("/api/deleteEmployee", deleteEmployeeHandler)
	http.HandleFunc("/api/saveWorkRecord", saveWorkRecordHandler)
	http.HandleFunc("/api/jobTypes", jobTypesHandler)
	http.HandleFunc("/api/addJobType", addJobTypeHandler)
	http.HandleFunc("/api/deleteJobType", deleteJobTypeHandler)
	// 新規追加
	http.HandleFunc("/api/getOwner", getOwnerHandler)
	http.HandleFunc("/api/setOwner", setOwnerHandler)
	http.HandleFunc("/api/login", loginHandler)
	// ここで従業員詳細情報取得APIを登録
	http.HandleFunc("/api/employeeDetail", employeeDetailHandler)
	// ここでタイムレコーダー履歴取得APIを登録
	http.HandleFunc("/api/timeRecords", timeRecordsHandler)
	// 新規追加：メモ保存API
	http.HandleFunc("/api/saveMemo", saveMemoHandler)
	http.HandleFunc("/api/deleteTimeRecord", deleteTimeRecordHandler)
	http.HandleFunc("/api/inconsistencies", inconsistenciesHandler)

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
