package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// JobType 構造体
type JobType struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// 有給休暇付与履歴を記録する構造体
type PaidVacationHistory struct {
	ID              int       `json:"id"`
	EmployeeID      int       `json:"employee_id"`
	GrantDate       string    `json:"grant_date"`
	GrantedDays     int       `json:"granted_days"`
	RecordTimestamp time.Time `json:"record_timestamp"`
}

// 従業員詳細に過去の有給休暇付与履歴も含めるため、Employee構造体にフィールドを追加
type Employee struct {
	ID                     int                   `json:"id"`
	EmployeeNumber         string                `json:"employee_number"`
	Name                   string                `json:"name"`
	Job                    string                `json:"job"`
	JobCode                string                `json:"job_code"`
	MaxAttendanceCount     int                   `json:"max_attendance_count"`
	PaidVacationLimit      int                   `json:"paid_vacation_limit"`
	PaidVacationGrantDate  string                `json:"paid_vacation_grant_date"`
	CurrentAttendanceCount int                   `json:"current_attendance_count,omitempty"`
	PaidVacationCount      int                   `json:"paid_vacation_count,omitempty"`
	EmploymentType         string                `json:"employment_type"`
	HourlyWage             int                   `json:"hourly_wage"`
	PaidVacationHistory    []PaidVacationHistory `json:"paid_vacation_history,omitempty"`
	PaidVacationGrantLabel string                `json:"paid_vacation_grant_label,omitempty"`
	ValidPaidVacationCount int                   `json:"valid_paid_vacation_count,omitempty"`
}

// WorkRecord 構造体
type WorkRecord struct {
	EmployeeID    int    `json:"employee_id"`
	TargetDate    string `json:"target_date"`
	TargetTime    string `json:"target_time"`
	TargetType    string `json:"target_type"`
	Memo          string `json:"memo"`
	BreakDuration *int   `json:"break_duration,omitempty"`
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
// func saveWorkRecordHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}
// 	var record WorkRecord
// 	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
// 		http.Error(w, "Bad request", http.StatusBadRequest)
// 		log.Println("Decode Error:", err)
// 		return
// 	}
// 	var breakDuration interface{}
// 	if record.BreakDuration != nil {
// 		breakDuration = *record.BreakDuration
// 	} else {
// 		breakDuration = nil
// 	}
// 	query := "INSERT INTO work_records (employee_id, target_date, target_time, target_type, break_duration) VALUES (?, ?, ?, ?, ?)"
// 	_, err := db.Exec(query, record.EmployeeID, record.TargetDate, record.TargetTime, record.TargetType, breakDuration)
// 	if err != nil {
// 		http.Error(w, "Database error", http.StatusInternalServerError)
// 		log.Println("Insert Error:", err)
// 		return
// 	}
// 	w.Header().Set("Content-Type", "text/plain")
// 	w.Write([]byte("Record saved successfully!"))
// }

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
	query := "INSERT INTO employees (name, job, job_code, max_attendance_count, paid_vacation_limit, paid_vacation_grant_date, employment_type, hourly_wage) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	result, err := db.Exec(query, emp.Name, emp.Job, jobCode, emp.MaxAttendanceCount, emp.PaidVacationLimit, emp.PaidVacationGrantDate, emp.EmploymentType, emp.HourlyWage)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Insert Error:", err)
		return
	}
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		http.Error(w, "Failed to get last insert ID", http.StatusInternalServerError)
		log.Println("Failed to get last insert ID:", err)
		return
	}
	employeeNumber := fmt.Sprintf("%s%06d", jobCode, lastInsertID)
	updateQuery := "UPDATE employees SET employee_number = ? WHERE id = ?"
	_, err = db.Exec(updateQuery, employeeNumber, lastInsertID)
	if err != nil {
		http.Error(w, "Failed to update employee number", http.StatusInternalServerError)
		log.Println("Update Error:", err)
		return
	}
	newEmployee := Employee{
		EmployeeNumber:        employeeNumber,
		Name:                  emp.Name,
		Job:                   emp.Job,
		JobCode:               jobCode,
		MaxAttendanceCount:    emp.MaxAttendanceCount,
		PaidVacationLimit:     emp.PaidVacationLimit,
		PaidVacationGrantDate: emp.PaidVacationGrantDate,
		EmploymentType:        emp.EmploymentType,
		HourlyWage:            emp.HourlyWage,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newEmployee)
}

// 職種一覧取得API
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

// 職種追加API
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

// 従業員削除API
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

// 職種削除API
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

// オーナー情報構造体
type OwnerInfo struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// オーナー情報取得API
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
		Password: password.String,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(owner)
}

// オーナー情報設定API
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

// ログインAPI用構造体
type LoginRequest struct {
	Password string `json:"password"`
}

// ログインAPI
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
	if storedPassword.Valid && storedPassword.String == req.Password {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Login success"))
	} else {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
	}
}

// 従業員詳細情報取得API（修正済み）
// 従業員詳細情報取得API（雇用形態、時給、paid_vacation_grant_date の自動更新対応）
// 従業員詳細情報取得API（修正済み）
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

	// 従業員詳細の取得
	var emp Employee
	err := db.QueryRow(`
        SELECT id, employee_number, name, job, job_code, max_attendance_count, 
               paid_vacation_limit, paid_vacation_grant_date, employment_type, hourly_wage
        FROM employees WHERE employee_number = ?`, empNumber).
		Scan(&emp.ID, &emp.EmployeeNumber, &emp.Name, &emp.Job, &emp.JobCode, &emp.MaxAttendanceCount,
			&emp.PaidVacationLimit, &emp.PaidVacationGrantDate, &emp.EmploymentType, &emp.HourlyWage)
	if err != nil {
		http.Error(w, "Employee not found", http.StatusNotFound)
		return
	}

	// 現在の日時をローカルタイムで取得
	now := time.Now().Local()
	var grantDate time.Time
	// まずRFC3339形式でパースを試みる
	grantDate, err = time.Parse(time.RFC3339, emp.PaidVacationGrantDate)
	if err != nil {
		// 失敗した場合、「2006-01-02」形式でもパースする
		grantDate, err = time.Parse("2006-01-02", emp.PaidVacationGrantDate)
		if err != nil {
			log.Println("Failed to parse paid_vacation_grant_date for employee:", emp.ID, err)
		}
	}

	if err == nil {
		if now.Before(grantDate) {
			emp.PaidVacationGrantLabel = "有給休暇付与予定日"
		} else {
			emp.PaidVacationGrantLabel = "有給休暇付与日"

			// paid_vacation_limitが未設定（-1）なら0として記録する
			grantedDays := emp.PaidVacationLimit
			if grantedDays == -1 {
				grantedDays = 0
			}
			_, err := db.Exec("INSERT INTO paid_vacation_history (employee_id, grant_date, granted_days) VALUES (?, ?, ?)",
				emp.ID, grantDate.Format("2006-01-02"), grantedDays)
			if err != nil {
				log.Println("Failed to record paid vacation history for employee:", emp.ID, err)
			}

			newGrantDate := grantDate.AddDate(1, 0, 0)

			// 次回付与日の更新時、paid_vacation_limitを0に設定する
			_, err = db.Exec("UPDATE employees SET paid_vacation_grant_date = ?, paid_vacation_limit = 0 WHERE id = ?",
				newGrantDate.Format("2006-01-02"), emp.ID)
			if err != nil {
				log.Println("Failed to update paid vacation grant date for employee:", emp.ID, err)
			} else {
				emp.PaidVacationGrantDate = newGrantDate.Format("2006-01-02")
				emp.PaidVacationLimit = 0
			}
		}
	} else {
		log.Println("Failed to parse paid_vacation_grant_date for employee:", emp.ID, err)
	}

	// 有効な有給休暇日数を取得（2年以内の付与分を合計）
	twoYearsAgo := now.AddDate(-2, 0, 0).Format("2006-01-02")
	err = db.QueryRow(`
        SELECT COALESCE(SUM(granted_days), 0) 
        FROM paid_vacation_history 
        WHERE employee_id = ? AND grant_date >= ?`, emp.ID, twoYearsAgo).
		Scan(&emp.ValidPaidVacationCount)
	if err != nil {
		log.Println("Failed to fetch valid paid vacation count for employee:", emp.ID, err)
		emp.ValidPaidVacationCount = 0
	}

	// 従業員の有給休暇付与履歴を取得
	histories, err := getPaidVacationHistory(emp.ID)
	if err != nil {
		log.Println("Failed to fetch paid vacation history for employee:", emp.ID, err)
	} else {
		emp.PaidVacationHistory = histories
	}

	// 出勤回数取得
	var clockInCount int
	err = db.QueryRow("SELECT COUNT(*) FROM work_records WHERE employee_id = ? AND target_type = 'clock_in'", emp.ID).Scan(&clockInCount)
	if err != nil {
		clockInCount = 0
	}
	emp.CurrentAttendanceCount = clockInCount

	// 有給休暇取得数取得
	var paidVacationCount int
	err = db.QueryRow("SELECT COUNT(*) FROM work_records WHERE employee_id = ? AND target_type = 'paid_vacation'", emp.ID).Scan(&paidVacationCount)
	if err != nil {
		paidVacationCount = 0
	}
	emp.PaidVacationCount = paidVacationCount

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(emp)
}

// タイムレコーダー履歴取得API
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
	var empID int
	err := db.QueryRow("SELECT id FROM employees WHERE employee_number = ?", empNumber).Scan(&empID)
	if err != nil {
		http.Error(w, "Employee not found", http.StatusNotFound)
		return
	}
	month := r.URL.Query().Get("month")
	var rows *sql.Rows
	if month != "" {
		rows, err = db.Query(
			"SELECT employee_id, target_date, target_time, target_type, memo, break_duration FROM work_records WHERE employee_id = ? AND target_date LIKE ? ORDER BY target_date ASC, target_time ASC",
			empID, month+"%",
		)
	} else {
		rows, err = db.Query(
			"SELECT employee_id, target_date, target_time, target_type, memo, break_duration FROM work_records WHERE employee_id = ? ORDER BY target_date ASC, target_time ASC",
			empID,
		)
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
		if err := rows.Scan(&rec.EmployeeID, &rec.TargetDate, &rec.TargetTime, &rec.TargetType, &rec.Memo, &rec.BreakDuration); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Scan Error in timeRecordsHandler:", err)
			return
		}
		records = append(records, rec)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// メモ保存API
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

// タイムレコーダー記録削除API
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

// 不整合チェック用構造体
type Inconsistency struct {
	EmployeeID     int      `json:"employee_id"`
	EmployeeNumber string   `json:"employee_number"`
	EmployeeName   string   `json:"employee_name"`
	Date           string   `json:"date"`
	Issues         []string `json:"issues"`
}

// 不整合チェック関数
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
	if len(clockIns) > 0 && len(clockOuts) == 0 {
		issues = append(issues, "出勤のみ（退勤記録なし）")
	}
	if len(clockOuts) > 0 && len(clockIns) == 0 {
		issues = append(issues, "退勤のみ（出勤記録なし）")
	}
	if len(clockIns) > 1 {
		issues = append(issues, "複数の出勤記録")
	}
	if len(clockOuts) > 1 {
		issues = append(issues, "複数の退勤記録")
	}
	if len(breakStarts) != len(breakEnds) {
		issues = append(issues, "休憩記録不整合（開始と終了の数が一致しない）")
	}
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
	for i := 1; i < len(records); i++ {
		if records[i].TargetType == records[i-1].TargetType {
			issues = append(issues, "連続した同種の打刻")
			break
		}
	}
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

// 不整合取得API
func inconsistenciesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	today := time.Now()
	cutoff := today.Format("2006-01-02")
	rows, err := db.Query("SELECT employee_id, target_date, target_time, target_type, memo FROM work_records WHERE target_date < ? ORDER BY target_date ASC, target_time ASC", cutoff)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query Error in inconsistenciesHandler:", err)
		return
	}
	defer rows.Close()
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

type UpdateEmployeePayload struct {
	ID                    int    `json:"id"`
	Name                  string `json:"name"`
	Job                   string `json:"job"`
	MaxAttendanceCount    int    `json:"max_attendance_count"`
	PaidVacationLimit     int    `json:"paid_vacation_limit"`
	PaidVacationGrantDate string `json:"paid_vacation_grant_date"`
	EmploymentType        string `json:"employment_type"` // 追加
	HourlyWage            int    `json:"hourly_wage"`     // 追加
}

func updateEmployeeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload UpdateEmployeePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error in /api/updateEmployee:", err)
		return
	}
	var jobCode string
	err := db.QueryRow("SELECT code FROM job_types WHERE name = ?", payload.Job).Scan(&jobCode)
	if err == sql.ErrNoRows {
		http.Error(w, "Invalid job name", http.StatusBadRequest)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Database error in updateEmployeeHandler:", err)
		return
	}
	// 更新後の従業員番号を生成（職種コード＋ID）
	employeeNumber := fmt.Sprintf("%s%06d", jobCode, payload.ID)
	query := `
        UPDATE employees
        SET name = ?, job = ?, job_code = ?, max_attendance_count = ?, 
            paid_vacation_limit = ?, paid_vacation_grant_date = ?, employment_type = ?, hourly_wage = ?, employee_number = ?
        WHERE id = ?
    `
	_, err = db.Exec(query, payload.Name, payload.Job, jobCode, payload.MaxAttendanceCount,
		payload.PaidVacationLimit, payload.PaidVacationGrantDate, payload.EmploymentType, payload.HourlyWage, employeeNumber, payload.ID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Update Error in /api/updateEmployee:", err)
		return
	}
	var updated Employee
	err = db.QueryRow(`
        SELECT id, employee_number, name, job, job_code, max_attendance_count, 
               paid_vacation_limit, paid_vacation_grant_date, employment_type, hourly_wage
        FROM employees WHERE id = ?`, payload.ID).
		Scan(&updated.ID, &updated.EmployeeNumber, &updated.Name, &updated.Job,
			&updated.JobCode, &updated.MaxAttendanceCount, &updated.PaidVacationLimit, &updated.PaidVacationGrantDate, &updated.EmploymentType, &updated.HourlyWage)
	if err != nil {
		http.Error(w, "Failed to fetch updated employee", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

type MonthlySummary struct {
	EmpId                    int
	EmpName                  string
	TotalWorkMin             int
	TotalNightShiftMin       int
	AttendanceDays           int
	RemainingAttendanceCount int
	HolidayWorkMin           int
	PaidVacationTaken        int
	RemainingPaidVacation    int
}

type JobSummary struct {
	JobCode                  string
	JobName                  string
	TotalWorkMin             int
	TotalNightShiftMin       int
	AttendanceDays           int
	RemainingAttendanceCount int
	HolidayWorkMin           int
	PaidVacationTaken        int
	RemainingPaidVacation    int
}

type ReportData struct {
	MonthlySummary     []MonthlySummary
	JobSummary         []JobSummary
	SelectedReportType string
}

func getMonthlySummary() ([]MonthlySummary, error) {
	query := `
	SELECT e.id, e.name, COUNT(*) AS attendance_days
	FROM employees e
	JOIN work_records w ON e.id = w.employee_id
	WHERE w.target_type = 'clock_in'
	  AND MONTH(w.target_date) = MONTH(CURRENT_DATE())
	  AND YEAR(w.target_date) = YEAR(CURRENT_DATE())
	GROUP BY e.id, e.name
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var summaries []MonthlySummary
	for rows.Next() {
		var s MonthlySummary
		var days int
		if err := rows.Scan(&s.EmpId, &s.EmpName, &days); err != nil {
			return nil, err
		}
		s.AttendanceDays = days
		s.TotalWorkMin = days * 480
		s.TotalNightShiftMin = 0
		s.RemainingAttendanceCount = 20 - days
		s.HolidayWorkMin = 0
		s.PaidVacationTaken = 0
		s.RemainingPaidVacation = 10
		summaries = append(summaries, s)
	}
	return summaries, nil
}

func getMonthlySummaryByJob() ([]JobSummary, error) {
	query := `
	SELECT e.job_code, e.job, COUNT(*) AS attendance_days
	FROM employees e
	JOIN work_records w ON e.id = w.employee_id
	WHERE w.target_type = 'clock_in'
	  AND MONTH(w.target_date) = MONTH(CURRENT_DATE())
	  AND YEAR(w.target_date) = YEAR(CURRENT_DATE())
	GROUP BY e.job_code, e.job
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var summaries []JobSummary
	for rows.Next() {
		var s JobSummary
		var days int
		if err := rows.Scan(&s.JobCode, &s.JobName, &days); err != nil {
			return nil, err
		}
		s.AttendanceDays = days
		s.TotalWorkMin = days * 480
		s.TotalNightShiftMin = 0
		s.RemainingAttendanceCount = 20 - days
		s.HolidayWorkMin = 0
		s.PaidVacationTaken = 0
		s.RemainingPaidVacation = 10
		summaries = append(summaries, s)
	}
	return summaries, nil
}

func monthlySummaryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	summaries, err := getMonthlySummary()
	if err != nil {
		http.Error(w, "Failed to get monthly summary", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

func jobSummaryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	summaries, err := getMonthlySummaryByJob()
	if err != nil {
		http.Error(w, "Failed to get job summary", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

// 【新規追加】タイムレコーダー記録更新API
func updateWorkRecordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		EmployeeID    int    `json:"employee_id"`
		OldTargetDate string `json:"old_target_date"`
		OldTargetTime string `json:"old_target_time"` // ※break_durationの場合、この値は使わない
		TargetType    string `json:"target_type"`
		NewTargetDate string `json:"new_target_date"` // ※通常更新の場合使用
		NewTargetTime string `json:"new_target_time"` // ※通常更新の場合は時刻、break_durationの場合は新しい分数の文字列
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error in /api/updateWorkRecord:", err)
		return
	}

	// 休憩時間の場合は、break_durationカラムを更新する
	if req.TargetType == "break_duration" {
		// 休憩時間は分数として扱うので、NewTargetTimeは分数の文字列で送られてくる
		newMinutes, err := strconv.Atoi(req.NewTargetTime)
		if err != nil {
			http.Error(w, "Invalid break duration", http.StatusBadRequest)
			return
		}
		// ここでは、target_timeは固定値"00:00"となっている前提で更新する
		result, err := db.Exec("UPDATE work_records SET break_duration = ? WHERE employee_id = ? AND target_date = ? AND target_type = ?",
			newMinutes, req.EmployeeID, req.OldTargetDate, req.TargetType)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Update Error in /api/updateWorkRecord (break_duration):", err)
			return
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil || rowsAffected == 0 {
			http.Error(w, "Record not found or update failed", http.StatusNotFound)
			return
		}
	} else {
		// 通常の更新（出勤、退勤、休憩開始、休憩終了）では、target_dateとtarget_timeでレコードを特定
		result, err := db.Exec("UPDATE work_records SET target_date = ?, target_time = ? WHERE employee_id = ? AND target_date = ? AND target_time = ? AND target_type = ?",
			req.NewTargetDate, req.NewTargetTime, req.EmployeeID, req.OldTargetDate, req.OldTargetTime, req.TargetType)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Update Error in /api/updateWorkRecord:", err)
			return
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil || rowsAffected == 0 {
			http.Error(w, "Record not found or update failed", http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("タイムレコーダー記録を更新しました"))
}

func getPaidVacationHistory(employeeID int) ([]PaidVacationHistory, error) {
	rows, err := db.Query("SELECT id, employee_id, grant_date, granted_days, record_timestamp FROM paid_vacation_history WHERE employee_id = ? ORDER BY grant_date DESC", employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var histories []PaidVacationHistory
	for rows.Next() {
		var h PaidVacationHistory
		if err := rows.Scan(&h.ID, &h.EmployeeID, &h.GrantDate, &h.GrantedDays, &h.RecordTimestamp); err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}
	return histories, nil
}

// 有給休暇を使用する処理（FIFO順、2年以上前の分は無効）
func usePaidVacation(employeeID int, useDays int) error {
	// 現在のローカル時刻
	now := time.Now().Local()
	// 有効期限：2年前以降の付与のみを対象とする（例：2年前の日付）
	validSince := now.AddDate(-2, 0, 0).Format("2006-01-02")

	// 有効な有給休暇付与履歴を、古い付与日順に取得
	rows, err := db.Query(`
        SELECT id, granted_days 
        FROM paid_vacation_history 
        WHERE employee_id = ? AND grant_date >= ? 
        ORDER BY grant_date ASC
    `, employeeID, validSince)
	if err != nil {
		return fmt.Errorf("failed to query paid vacation history: %v", err)
	}
	defer rows.Close()

	// 合計利用可能日数を集計
	var totalAvailable int
	type recordInfo struct {
		id        int
		remaining int
	}
	var records []recordInfo
	for rows.Next() {
		var id, days int
		if err := rows.Scan(&id, &days); err != nil {
			return fmt.Errorf("failed to scan paid vacation history: %v", err)
		}
		if days > 0 {
			totalAvailable += days
			records = append(records, recordInfo{id: id, remaining: days})
		}
	}
	if totalAvailable < useDays {
		return fmt.Errorf("利用可能な有給休暇が不足しています（利用可能日数: %d, 使用日数: %d）", totalAvailable, useDays)
	}

	// 古いものから順に消化する
	daysToDeduct := useDays
	for _, rec := range records {
		if daysToDeduct <= 0 {
			break
		}
		var deduct int
		if rec.remaining >= daysToDeduct {
			deduct = daysToDeduct
		} else {
			deduct = rec.remaining
		}
		// 各レコードの granted_days を減算更新
		_, err := db.Exec("UPDATE paid_vacation_history SET granted_days = granted_days - ? WHERE id = ?", deduct, rec.id)
		if err != nil {
			return fmt.Errorf("failed to update paid vacation history record (id=%d): %v", rec.id, err)
		}
		daysToDeduct -= deduct
	}
	return nil
}

// 打刻情報保存API（paid_vacationの場合、有給休暇使用処理を追加）
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
	// 休憩時間の値
	var breakDuration interface{}
	if record.BreakDuration != nil {
		breakDuration = *record.BreakDuration
	} else {
		breakDuration = nil
	}

	// paid_vacationの場合、使用可能日数を消化する処理を実施（1打刻=1日使用とする）
	if record.TargetType == "paid_vacation" {
		// ここで有給休暇の使用処理を実行
		if err := usePaidVacation(record.EmployeeID, 1); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	query := "INSERT INTO work_records (employee_id, target_date, target_time, target_type, break_duration) VALUES (?, ?, ?, ?, ?)"
	_, err := db.Exec(query, record.EmployeeID, record.TargetDate, record.TargetTime, record.TargetType, breakDuration)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Insert Error:", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Record saved successfully!"))
}

// 有給休暇付与履歴更新API
func updatePaidVacationHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		ID          int `json:"id"`
		GrantedDays int `json:"granted_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	result, err := db.Exec("UPDATE paid_vacation_history SET granted_days = ? WHERE id = ?", payload.GrantedDays, payload.ID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Update Error in /api/updatePaidVacationHistory:", err)
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		http.Error(w, "Record not found or update failed", http.StatusNotFound)
		return
	}
	// 更新後のレコードを返す
	var updated PaidVacationHistory
	err = db.QueryRow("SELECT id, employee_id, grant_date, granted_days, record_timestamp FROM paid_vacation_history WHERE id = ?", payload.ID).
		Scan(&updated.ID, &updated.EmployeeID, &updated.GrantDate, &updated.GrantedDays, &updated.RecordTimestamp)
	if err != nil {
		http.Error(w, "Failed to fetch updated record", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func main() {
	initDB()

	// APIハンドラーを先に登録する
	http.HandleFunc("/api/employees", employeesHandler)
	http.HandleFunc("/api/addEmployee", addEmployeeHandler)
	http.HandleFunc("/api/deleteEmployee", deleteEmployeeHandler)
	http.HandleFunc("/api/saveWorkRecord", saveWorkRecordHandler)
	http.HandleFunc("/api/jobTypes", jobTypesHandler)
	http.HandleFunc("/api/addJobType", addJobTypeHandler)
	http.HandleFunc("/api/deleteJobType", deleteJobTypeHandler)
	http.HandleFunc("/api/getOwner", getOwnerHandler)
	http.HandleFunc("/api/setOwner", setOwnerHandler)
	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/employeeDetail", employeeDetailHandler)
	http.HandleFunc("/api/timeRecords", timeRecordsHandler)
	http.HandleFunc("/api/saveMemo", saveMemoHandler)
	http.HandleFunc("/api/deleteTimeRecord", deleteTimeRecordHandler)
	http.HandleFunc("/api/inconsistencies", inconsistenciesHandler)
	http.HandleFunc("/api/updateEmployee", updateEmployeeHandler)
	http.HandleFunc("/api/updateWorkRecord", updateWorkRecordHandler)
	http.HandleFunc("/api/monthlySummary", monthlySummaryHandler)
	http.HandleFunc("/api/jobSummary", jobSummaryHandler)
	http.HandleFunc("/api/updatePaidVacationHistory", updatePaidVacationHistoryHandler)

	// その後、静的ファイルサーバーを登録する
	http.Handle("/", http.FileServer(http.Dir("./static")))

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
