package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"

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
	TransportationExpense  int                   `json:"transportation_expense"` // 追加
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

// initializeSecretAnswer は、system_config の secret_answer_hash が空の場合、デフォルト値をハッシュ化して保存します。
func initializeSecretAnswer() {
	var secretAnswerHash string
	err := db.QueryRow("SELECT secret_answer_hash FROM system_config WHERE id = 1").Scan(&secretAnswerHash)
	if err != nil {
		log.Fatal("Failed to query secret_answer_hash:", err)
	}
	if secretAnswerHash == "" {
		defaultSecret := "Pちゃん" // デフォルトの秘密の合言葉
		hashed, err := bcrypt.GenerateFromPassword([]byte(defaultSecret), bcrypt.DefaultCost)
		if err != nil {
			log.Fatal("Failed to hash default secret answer:", err)
		}
		_, err = db.Exec("UPDATE system_config SET secret_answer_hash = ? WHERE id = 1", string(hashed))
		if err != nil {
			log.Fatal("Failed to update secret_answer_hash:", err)
		}
		fmt.Println("Initialized secret_answer_hash with default value")
	}
}

// 従業員情報を取得するAPI
func employeesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := db.Query(`
    SELECT e.employee_number, e.name, e.job, e.job_code, e.max_attendance_count, 
           e.paid_vacation_limit, e.paid_vacation_grant_date, e.employment_type
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
		if err := rows.Scan(&emp.EmployeeNumber, &emp.Name, &emp.Job, &emp.JobCode,
			&emp.MaxAttendanceCount, &emp.PaidVacationLimit, &emp.PaidVacationGrantDate, &emp.EmploymentType); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Scan Error:", err)
			return
		}
		employees = append(employees, emp)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(employees)
}

// 従業員追加API
func addEmployeeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var emp Employee
	if err := json.NewDecoder(r.Body).Decode(&emp); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// 従業員名の重複チェック
	var existingCount int
	err := db.QueryRow("SELECT COUNT(*) FROM employees WHERE name = ?", emp.Name).Scan(&existingCount)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Database error:", err)
		return
	}
	if existingCount > 0 {
		http.Error(w, "従業員名が既に存在します", http.StatusBadRequest)
		return
	}

	var jobCode string
	err = db.QueryRow("SELECT code FROM job_types WHERE name = ?", emp.Job).Scan(&jobCode)
	if err == sql.ErrNoRows {
		http.Error(w, "Invalid job name", http.StatusBadRequest)
		log.Println("Invalid job name:", emp.Job)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Database error:", err)
		return
	}

	query := "INSERT INTO employees (name, job, job_code, max_attendance_count, paid_vacation_limit, paid_vacation_grant_date, employment_type, hourly_wage, transportation_expense) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	result, err := db.Exec(query, emp.Name, emp.Job, jobCode, emp.MaxAttendanceCount, emp.PaidVacationLimit, emp.PaidVacationGrantDate, emp.EmploymentType, emp.HourlyWage, emp.TransportationExpense)
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
		TransportationExpense: emp.TransportationExpense,
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

	// bcrypt で新しいパスワードをハッシュ化
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(info.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error processing password", http.StatusInternalServerError)
		log.Println("bcrypt error:", err)
		return
	}

	query := `
		UPDATE system_config
		SET owner_email = ?, owner_password = ?
		WHERE id = 1
	`
	_, err = db.Exec(query, info.Email, string(hashedPassword))
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Update Error:", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("新たなパスワードを設定しました"))
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
	var storedHash string
	err := db.QueryRow("SELECT owner_password FROM system_config WHERE id = 1").Scan(&storedHash)
	if err == sql.ErrNoRows {
		http.Error(w, "No password set", http.StatusUnauthorized)
		log.Println("No password found in system_config")
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query Error in /api/login:", err)
		return
	}

	// bcrypt を使ってハッシュと入力パスワードを照合
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Login success"))
}

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
           paid_vacation_limit, paid_vacation_grant_date, employment_type, hourly_wage, transportation_expense
    FROM employees WHERE employee_number = ?`, empNumber).
		Scan(&emp.ID, &emp.EmployeeNumber, &emp.Name, &emp.Job, &emp.JobCode, &emp.MaxAttendanceCount,
			&emp.PaidVacationLimit, &emp.PaidVacationGrantDate, &emp.EmploymentType, &emp.HourlyWage, &emp.TransportationExpense)
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
			`SELECT employee_id, 
			        DATE_FORMAT(target_date, '%Y-%m-%d') as target_date, 
			        DATE_FORMAT(target_time, '%H:%i:%s') as target_time, 
			        target_type, 
			        memo, 
			        break_duration 
			 FROM work_records 
			 WHERE employee_id = ? 
			   AND DATE_FORMAT(target_date, '%Y-%m') = ? 
			 ORDER BY target_date ASC, target_time ASC`,
			empID, month,
		)
	} else {
		rows, err = db.Query(
			`SELECT employee_id, 
			        DATE_FORMAT(target_date, '%Y-%m-%d') as target_date, 
			        DATE_FORMAT(target_time, '%H:%i:%s') as target_time, 
			        target_type, 
			        memo, 
			        break_duration 
			 FROM work_records 
			 WHERE employee_id = ? 
			 ORDER BY target_date ASC, target_time ASC`,
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
	EmploymentType        string `json:"employment_type"`        // 追加
	HourlyWage            int    `json:"hourly_wage"`            // 追加
	TransportationExpense int    `json:"transportation_expense"` // 追加
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
	// 交通費も更新するため、SQL文を変更
	query := `
        UPDATE employees
        SET name = ?, job = ?, job_code = ?, max_attendance_count = ?, 
            paid_vacation_limit = ?, paid_vacation_grant_date = ?, employment_type = ?, hourly_wage = ?, transportation_expense = ?, employee_number = ?
        WHERE id = ?
    `
	_, err = db.Exec(query, payload.Name, payload.Job, jobCode, payload.MaxAttendanceCount,
		payload.PaidVacationLimit, payload.PaidVacationGrantDate, payload.EmploymentType, payload.HourlyWage, payload.TransportationExpense, employeeNumber, payload.ID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Update Error in /api/updateEmployee:", err)
		return
	}
	var updated Employee
	err = db.QueryRow(`
        SELECT id, employee_number, name, job, job_code, max_attendance_count, 
               paid_vacation_limit, paid_vacation_grant_date, employment_type, hourly_wage, transportation_expense
        FROM employees WHERE id = ?`, payload.ID).
		Scan(&updated.ID, &updated.EmployeeNumber, &updated.Name, &updated.Job,
			&updated.JobCode, &updated.MaxAttendanceCount, &updated.PaidVacationLimit, &updated.PaidVacationGrantDate, &updated.EmploymentType, &updated.HourlyWage, &updated.TransportationExpense)
	if err != nil {
		http.Error(w, "Failed to fetch updated employee", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// MonthlySummary 構造体（有給取得日数フィールドを追加）
// MonthlySummary 構造体（従業員IDフィールド追加）
type MonthlySummary struct {
	EmpID                 int    `json:"emp_id"`
	EmpNumber             string `json:"emp_number"`
	EmpName               string `json:"emp_name"`
	EmploymentType        string `json:"employment_type"` // 追加
	HourlyWage            int    `json:"hourly_wage"`
	TransportationExpense int    `json:"transportation_expense"`
	TotalWorkMin          int    `json:"total_work_min"`
	TotalNightShiftMin    int    `json:"total_night_shift_min"`
	AttendanceDays        int    `json:"attendance_days"`
	HolidayWorkMin        int    `json:"holiday_work_min"` // 固定値例
	PaidVacationTaken     int    `json:"paid_vacation_taken"`
	RemainingPaidVacation int    `json:"remaining_paid_vacation"` // 固定値例
	MonthlySalary         int    `json:"monthly_salary"`
	Memo                  string `json:"memo"`
}

// JobSummary は職種別の集計結果を表します。
type JobSummary struct {
	JobCode            string `json:"job_code"`
	JobName            string `json:"job_name"`
	TotalWorkMin       int    `json:"total_work_min"`
	TotalNightShiftMin int    `json:"total_night_shift_min"`
	MonthlySalary      int    `json:"monthly_salary"`
}

type ReportData struct {
	MonthlySummary     []MonthlySummary
	JobSummary         []JobSummary
	SelectedReportType string
}

// 対象の年月（year, month）ごとに、各従業員の勤務日ごとに
// 勤務時間（diff）と、夜勤時間（nightDiff）を算出し、全体を集計する関数
func getMonthlySummary(year, month int) ([]MonthlySummary, error) {
	monthStr := fmt.Sprintf("%04d-%02d", year, month)

	query := `
    SELECT 
        e.employee_number, 
        e.name, 
        e.id, 
        e.employment_type,
        e.hourly_wage, 
        e.transportation_expense,
        COALESCE(SUM(wrd.work_diff), 0) AS total_work_min,
        COALESCE(SUM(wrd.night_diff), 0) AS total_night_shift_min,
        COALESCE(SUM(wrd.extra_min), 0) AS total_extra_min,
        COALESCE(SUM(wrd.break_duration), 0) AS total_break_min,
        COUNT(DISTINCT wrd.target_date) AS attendance_days,
        (
            SELECT COUNT(*)
            FROM work_records pv
            WHERE pv.employee_id = e.id
              AND pv.target_type = 'paid_vacation'
              AND MONTH(pv.target_date) = ?
              AND YEAR(pv.target_date) = ?
        ) AS paid_vacation_taken,
        IFNULL(MAX(m.memo), '') AS memo
    FROM employees e
    LEFT JOIN monthly_memos m 
        ON e.id = m.employee_id AND m.month = ?
    LEFT JOIN (
        SELECT 
            employee_id, 
            target_date,
            TIMESTAMPDIFF(MINUTE,
                CONCAT(target_date, ' ', MIN(CASE WHEN target_type = 'clock_in' THEN target_time END)),
                CONCAT(target_date, ' ', MAX(CASE WHEN target_type = 'clock_out' THEN target_time END))
            ) AS work_diff,
            CASE 
                WHEN TIMESTAMPDIFF(MINUTE,
                        CONCAT(target_date, ' ', MIN(CASE WHEN target_type = 'clock_in' THEN target_time END)),
                        CONCAT(target_date, ' ', MAX(CASE WHEN target_type = 'clock_out' THEN target_time END))
                     ) - COALESCE(SUM(CASE WHEN target_type = 'break_duration' THEN break_duration ELSE 0 END), 0) > 480 
                THEN TIMESTAMPDIFF(MINUTE,
                        CONCAT(target_date, ' ', MIN(CASE WHEN target_type = 'clock_in' THEN target_time END)),
                        CONCAT(target_date, ' ', MAX(CASE WHEN target_type = 'clock_out' THEN target_time END))
                     ) - COALESCE(SUM(CASE WHEN target_type = 'break_duration' THEN break_duration ELSE 0 END), 0) - 480
                ELSE 0
            END AS extra_min,
            CASE
                WHEN MAX(CASE WHEN target_type = 'clock_out' THEN target_time END) < MIN(CASE WHEN target_type = 'clock_in' THEN target_time END)
                THEN
                    GREATEST(0, TIMESTAMPDIFF(MINUTE,
                        CONCAT(target_date, ' ', '22:00:00'),
                        CONCAT(target_date, ' ', '24:00:00')
                    ) - 
                    CASE WHEN MIN(CASE WHEN target_type = 'clock_in' THEN target_time END) > '22:00:00'
                         THEN TIMESTAMPDIFF(MINUTE,
                             CONCAT(target_date, ' ', MIN(CASE WHEN target_type = 'clock_in' THEN target_time END)),
                             CONCAT(target_date, ' ', '24:00:00')
                         )
                         ELSE 0
                    END)
                    +
                    GREATEST(0, TIMESTAMPDIFF(MINUTE,
                        CONCAT(DATE_ADD(target_date, INTERVAL 1 DAY), ' ', '00:00:00'),
                        CONCAT(DATE_ADD(target_date, INTERVAL 1 DAY), ' ', LEAST(MAX(CASE WHEN target_type = 'clock_out' THEN target_time END), '05:00:00'))
                    ))
                ELSE
                    (
                        CASE
                            WHEN MAX(CASE WHEN target_type = 'clock_out' THEN target_time END) > '22:00:00'
                                THEN TIMESTAMPDIFF(MINUTE,
                                     CONCAT(target_date, ' ', '22:00:00'),
                                     CONCAT(target_date, ' ', LEAST(MAX(CASE WHEN target_type = 'clock_out' THEN target_time END), '24:00:00'))
                                 )
                            ELSE 0
                        END
                    )
                    +
                    (
                        CASE
                            WHEN MIN(CASE WHEN target_type = 'clock_in' THEN target_time END) < '05:00:00'
                                THEN 
                                    CASE
                                        WHEN MAX(CASE WHEN target_type = 'clock_out' THEN target_time END) <= '05:00:00'
                                            THEN TIMESTAMPDIFF(MINUTE,
                                                 CONCAT(target_date, ' ', MIN(CASE WHEN target_type = 'clock_in' THEN target_time END)),
                                                 CONCAT(target_date, ' ', MAX(CASE WHEN target_type = 'clock_out' THEN target_time END))
                                             )
                                        WHEN MAX(CASE WHEN target_type = 'clock_out' THEN target_time END) > '05:00:00'
                                            THEN TIMESTAMPDIFF(MINUTE,
                                                 CONCAT(target_date, ' ', MIN(CASE WHEN target_type = 'clock_in' THEN target_time END)),
                                                 CONCAT(target_date, ' ', '05:00:00')
                                             )
                                        ELSE 0
                                    END
                            ELSE 0
                        END
                    )
            END AS night_diff,
            COALESCE(SUM(CASE WHEN target_type = 'break_duration' THEN break_duration ELSE 0 END), 0) AS break_duration
        FROM work_records
        WHERE target_type IN ('clock_in','clock_out', 'break_duration')
          AND MONTH(target_date) = ?
          AND YEAR(target_date) = ?
        GROUP BY employee_id, target_date
    ) wrd ON e.id = wrd.employee_id
    GROUP BY e.id, e.employee_number, e.name, e.employment_type, e.hourly_wage, e.transportation_expense, m.memo
    `

	rows, err := db.Query(query, month, year, monthStr, month, year)
	if err != nil {
		log.Printf("Query error in getMonthlySummary: %v", err)
		return nil, err
	}
	defer rows.Close()

	var summaries []MonthlySummary
	for rows.Next() {
		var s MonthlySummary
		var empID int
		var totalExtraMin int
		var totalBreakMin int
		var memo sql.NullString

		if err := rows.Scan(&s.EmpNumber, &s.EmpName, &empID, &s.EmploymentType, &s.HourlyWage, &s.TransportationExpense, &s.TotalWorkMin, &s.TotalNightShiftMin, &totalExtraMin, &totalBreakMin, &s.AttendanceDays, &s.PaidVacationTaken, &memo); err != nil {
			log.Printf("Scan error in getMonthlySummary: %v", err)
			return nil, err
		}

		s.EmpID = empID
		s.HolidayWorkMin = 0
		s.TotalWorkMin -= totalBreakMin // 休憩時間を引く

		if memo.Valid {
			s.Memo = memo.String
		} else {
			s.Memo = ""
		}

		// 残り有給休暇数の取得
		var validPaidVacation int
		err := db.QueryRow(`
            SELECT COALESCE(SUM(granted_days), 0)
            FROM paid_vacation_history
            WHERE employee_id = ?
            AND grant_date >= DATE_SUB(CURDATE(), INTERVAL 2 YEAR)
        `, empID).Scan(&validPaidVacation)
		if err != nil {
			log.Printf("Error getting valid paid vacation: %v", err)
			s.RemainingPaidVacation = 0
		} else {
			var totalUsed int
			err := db.QueryRow(`
                SELECT COUNT(*)
                FROM work_records
                WHERE employee_id = ?
                AND target_type = 'paid_vacation'
            `, empID).Scan(&totalUsed)
			if err != nil {
				log.Printf("Error getting total used vacation: %v", err)
				s.RemainingPaidVacation = validPaidVacation
			} else {
				s.RemainingPaidVacation = validPaidVacation - totalUsed
				if s.RemainingPaidVacation < 0 {
					s.RemainingPaidVacation = 0
				}
			}
		}

		// 月給計算
		workHours := float64(s.TotalWorkMin) / 60.0
		extraHours := float64(totalExtraMin) / 60.0
		print(totalExtraMin)
		nightHours := float64(s.TotalNightShiftMin) / 60.0
		monthlySalary := float64(s.HourlyWage) * (workHours + (extraHours+nightHours)*0.25)

		// 交通費を加算
		monthlySalary += float64(s.TransportationExpense * s.AttendanceDays)

		s.MonthlySalary = int(monthlySalary)

		summaries = append(summaries, s)
	}

	return summaries, nil
}

// getMonthlySummaryByJob は、指定年月の月次勤怠レポート（getMonthlySummary の結果）から
// 雇用形態が「パート」の従業員のみを対象に、職種コード（従業員番号の先頭2文字）ごとに
// 合計勤務時間、合計夜勤時間、月給を集計して返します。
func getMonthlySummaryByJob(year, month int) ([]JobSummary, error) {
	// まず、既存の getMonthlySummary を呼び出して当月の月次勤怠レポートを取得
	monthlySummaries, err := getMonthlySummary(year, month)
	if err != nil {
		return nil, err
	}

	// 集計用マップ：キーは職種コード（従業員番号の先頭2文字）
	aggregation := make(map[string]*JobSummary)
	for _, ms := range monthlySummaries {
		// 対象は雇用形態が「パート」のみ
		if ms.EmploymentType == "パート" {
			// 従業員番号の先頭2文字を職種コードとする（例："01"）
			if len(ms.EmpNumber) < 2 {
				continue // 想定外のデータはスキップ
			}
			jobCode := ms.EmpNumber[:2]
			js, exists := aggregation[jobCode]
			if !exists {
				js = &JobSummary{
					JobCode:            jobCode,
					TotalWorkMin:       0,
					TotalNightShiftMin: 0,
					MonthlySalary:      0,
				}
				aggregation[jobCode] = js
			}
			js.TotalWorkMin += ms.TotalWorkMin
			js.TotalNightShiftMin += ms.TotalNightShiftMin
			js.MonthlySalary += ms.MonthlySalary
		}
	}

	// job_types テーブルから、職種コード→職種名のマッピングを取得
	rows, err := db.Query("SELECT code, name FROM job_types")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobTypeMap := make(map[string]string)
	for rows.Next() {
		var code, name string
		if err := rows.Scan(&code, &name); err != nil {
			return nil, err
		}
		jobTypeMap[code] = name
	}

	// 集計結果に職種名を設定し、スライスに変換
	var jobSummaries []JobSummary
	for code, js := range aggregation {
		if jobName, ok := jobTypeMap[code]; ok {
			js.JobName = jobName
		} else {
			js.JobName = ""
		}
		jobSummaries = append(jobSummaries, *js)
	}

	return jobSummaries, nil
}

// monthlySummaryHandler は、クエリパラメータ "month" が指定されている場合、その年月のデータを抽出します
func monthlySummaryHandler(w http.ResponseWriter, r *http.Request) {
	monthStr := r.URL.Query().Get("month")
	var year, month int
	if monthStr != "" {
		parts := strings.Split(monthStr, "-")
		if len(parts) != 2 {
			http.Error(w, "Invalid month parameter", http.StatusBadRequest)
			return
		}
		var err error
		year, err = strconv.Atoi(parts[0])
		if err != nil {
			http.Error(w, "Invalid year in month parameter", http.StatusBadRequest)
			return
		}
		month, err = strconv.Atoi(parts[1])
		if err != nil {
			http.Error(w, "Invalid month in month parameter", http.StatusBadRequest)
			return
		}
	} else {
		now := time.Now()
		year = now.Year()
		month = int(now.Month())
	}

	summaries, err := getMonthlySummary(year, month)
	if err != nil {
		http.Error(w, "Failed to get monthly summary", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

func jobSummaryHandler(w http.ResponseWriter, r *http.Request) {
	// URLクエリパラメータ "month" を "YYYY-MM" 形式で取得（無い場合は当月）
	monthStr := r.URL.Query().Get("month")
	var year, month int
	if monthStr != "" {
		parts := strings.Split(monthStr, "-")
		if len(parts) != 2 {
			http.Error(w, "Invalid month parameter", http.StatusBadRequest)
			return
		}
		var err error
		year, err = strconv.Atoi(parts[0])
		if err != nil {
			http.Error(w, "Invalid year in month parameter", http.StatusBadRequest)
			return
		}
		month, err = strconv.Atoi(parts[1])
		if err != nil {
			http.Error(w, "Invalid month in month parameter", http.StatusBadRequest)
			return
		}
	} else {
		now := time.Now()
		year = now.Year()
		month = int(now.Month())
	}

	jobSummaries, err := getMonthlySummaryByJob(year, month)
	if err != nil {
		http.Error(w, "Failed to get job summary", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobSummaries)
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
		log.Println("Failed to decode request:", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	log.Printf("Received update request: %+v\n", req)

	var query string
	var args []interface{}

	// 休憩時間の場合は、break_durationカラムを更新する
	if req.TargetType == "break_duration" {
		// new_target_time を数値に変換
		breakDuration, err := strconv.Atoi(req.NewTargetTime)
		if err != nil {
			log.Println("Invalid break duration:", err)
			http.Error(w, "Invalid break duration", http.StatusBadRequest)
			return
		}
		query = "UPDATE work_records SET break_duration = ? WHERE employee_id = ? AND target_date = ? AND target_type = 'break_duration'"
		args = []interface{}{breakDuration, req.EmployeeID, req.OldTargetDate}
	} else {
		// 通常の更新（出勤、退勤、休憩開始、休憩終了）では、target_dateとtarget_timeでレコードを特定
		query = "UPDATE work_records SET target_date = ?, target_time = ? WHERE employee_id = ? AND target_date = ? AND target_time = ? AND target_type = ?"
		args = []interface{}{req.NewTargetDate, req.NewTargetTime, req.EmployeeID, req.OldTargetDate, req.OldTargetTime, req.TargetType}
	}

	log.Printf("Executing query: %s with args: %+v\n", query, args)

	_, err := db.Exec(query, args...)
	if err != nil {
		log.Println("Database error:", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
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
	now := time.Now().Local()
	validSince := now.AddDate(-2, 0, 0).Format("2006-01-02")

	// 2年以内に付与された合計有給日数を取得（履歴テーブルは更新しないので、ここは固定値）
	var totalGranted int
	err := db.QueryRow(`SELECT COALESCE(SUM(granted_days), 0) FROM paid_vacation_history WHERE employee_id = ? AND grant_date >= ?`, employeeID, validSince).Scan(&totalGranted)
	if err != nil {
		return fmt.Errorf("failed to fetch total granted vacation: %v", err)
	}

	// 使用済みの有給休暇（work_recordsテーブルの paid_vacation レコード件数）を取得
	var totalUsed int
	err = db.QueryRow(`SELECT COUNT(*) FROM work_records WHERE employee_id = ? AND target_type = 'paid_vacation'`, employeeID).Scan(&totalUsed)
	if err != nil {
		return fmt.Errorf("failed to fetch total used vacation: %v", err)
	}

	available := totalGranted - totalUsed
	if available < useDays {
		return fmt.Errorf("利用可能な有給休暇が不足しています（利用可能日数: %d, 使用日数: %d）", available, useDays)
	}
	// ここでは履歴テーブルは更新せず、利用可能な有給がある場合は nil を返す
	return nil
}

// 打刻情報保存API（paid_vacationの場合、有給休暇使用処理を追加）
// 打刻情報保存API（paid_vacationの場合、有給休暇使用処理を追加し、さらに
// 同じ日に複数回の出勤または退勤記録が登録されないようにチェックします）
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

	// 同じ日の打刻がすでに登録されているかチェック
	switch record.TargetType {
	// 出勤、退勤の場合は、その日の同じ種別の記録が既にあると拒否
	case "clock_in", "clock_out":
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM work_records WHERE employee_id = ? AND target_date = ? AND target_type = ?",
			record.EmployeeID, record.TargetDate, record.TargetType,
		).Scan(&count)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Clock in/out Check Error:", err)
			return
		}
		if count > 0 {
			http.Error(w, "同じ日に複数回の出勤または退勤記録はできません", http.StatusBadRequest)
			return
		}

	// 有給休暇の場合は、既にその日の有給休暇記録があるかチェック
	case "paid_vacation":
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM work_records WHERE employee_id = ? AND target_date = ? AND target_type = 'paid_vacation'",
			record.EmployeeID, record.TargetDate,
		).Scan(&count)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Paid Vacation Check Error:", err)
			return
		}
		if count > 0 {
			http.Error(w, "同じ日に有給休暇はすでに設定されています", http.StatusBadRequest)
			return
		}
		// 有給休暇が使用可能かチェック（履歴は更新せずに利用可能日数だけ確認）
		if err := usePaidVacation(record.EmployeeID, 1); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	// その他の記録（例：休憩開始、休憩終了、休憩時間など）
	default:
		// ※必要に応じて他の種別についてのチェックを追加可能
		// ここでは、有給休暇記録がある場合はその他の記録も追加できないという既存の仕様を維持
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM work_records WHERE employee_id = ? AND target_date = ? AND target_type = 'paid_vacation'",
			record.EmployeeID, record.TargetDate,
		).Scan(&count)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Println("Paid Vacation Check Error:", err)
			return
		}
		if count > 0 {
			http.Error(w, "有給休暇取得済みのため、その他の打刻は追加できません", http.StatusBadRequest)
			return
		}
	}

	// work_recordsテーブルに新規記録を追加
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

// 新規追加：秘密の合言葉を使ったパスワード再設定ハンドラー
func secretResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		NewPassword  string `json:"new_password"`
		SecretAnswer string `json:"secret_answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Println("Decode Error in /api/secretReset:", err)
		return
	}

	// データベースからハッシュ化された秘密の合言葉を取得する
	var storedSecretHash string
	err := db.QueryRow("SELECT secret_answer_hash FROM system_config WHERE id = 1").Scan(&storedSecretHash)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query Error in secretResetHandler:", err)
		return
	}

	// ユーザー入力の秘密の合言葉と保存済みのハッシュを比較する
	if err := bcrypt.CompareHashAndPassword([]byte(storedSecretHash), []byte(req.SecretAnswer)); err != nil {
		http.Error(w, "秘密の合言葉が間違っています", http.StatusUnauthorized)
		return
	}

	// 新しいパスワードをハッシュ化して更新
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error processing password", http.StatusInternalServerError)
		log.Println("bcrypt error in secretResetHandler:", err)
		return
	}
	_, err = db.Exec("UPDATE system_config SET owner_password = ? WHERE id = 1", string(hashedPassword))
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Update Error in /api/secretReset:", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("パスワードが再設定されました"))
}

func saveMonthlyMemoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		EmployeeID int    `json:"employee_id"`
		Month      string `json:"month"`
		Memo       string `json:"memo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// 例：INSERT ... ON DUPLICATE KEY UPDATE で保存
	query := `INSERT INTO monthly_memos (employee_id, month, memo)
              VALUES (?, ?, ?)
              ON DUPLICATE KEY UPDATE memo = VALUES(memo)`
	_, err := db.Exec(query, req.EmployeeID, req.Month, req.Memo)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Error saving monthly memo:", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("月次メモを保存しました"))
}

// getTimeRecords は、指定された従業員番号と年月の打刻記録を取得する関数
func getTimeRecords(empNumber string, monthStr string) ([]WorkRecord, error) {
	var empID int
	err := db.QueryRow("SELECT id FROM employees WHERE employee_number = ?", empNumber).Scan(&empID)
	if err != nil {
		return nil, fmt.Errorf("employee not found: %v", err)
	}

	rows, err := db.Query(
		`SELECT employee_id, 
		        DATE_FORMAT(target_date, '%Y-%m-%d') as target_date, 
		        DATE_FORMAT(target_time, '%H:%i:%s') as target_time, 
		        target_type, 
		        memo, 
		        break_duration 
		 FROM work_records 
		 WHERE employee_id = ? 
		   AND DATE_FORMAT(target_date, '%Y-%m') = ? 
		 ORDER BY target_date ASC, target_time ASC`,
		empID, monthStr,
	)
	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}
	defer rows.Close()

	var records []WorkRecord
	for rows.Next() {
		var rec WorkRecord
		if err := rows.Scan(&rec.EmployeeID, &rec.TargetDate, &rec.TargetTime, &rec.TargetType, &rec.Memo, &rec.BreakDuration); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		records = append(records, rec)
	}

	return records, nil
}

// 指定の年月からその月の日数を取得する関数
func getDaysInMonth(year int, month int) int {
	// うるう年対応
	if month == 2 {
		if (year%4 == 0 && year%100 != 0) || (year%400 == 0) {
			return 29
		}
		return 28
	}
	// 30日の月
	if month == 4 || month == 6 || month == 9 || month == 11 {
		return 30
	}
	return 31 // 31日の月
}

// getYearMonthFromStr は "YYYY-MM" の文字列から年と月を取得する関数
func getYearMonthFromStr(monthStr string) (int, int, error) {
	parts := strings.Split(monthStr, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid month format: %s", monthStr)
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid year format: %s", parts[0])
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid month format: %s", parts[1])
	}

	// 月の範囲チェック
	if month < 1 || month > 12 {
		return 0, 0, fmt.Errorf("invalid month value: %d", month)
	}

	return year, month, nil
}

// "YYYY-MM" の文字列から年と月を取得する関数
func exportExcelHandler(w http.ResponseWriter, r *http.Request) {
	monthStr := r.URL.Query().Get("month")
	if monthStr == "" {
		http.Error(w, "Missing month parameter", http.StatusBadRequest)
		return
	}

	year, month, err := getYearMonthFromStr(monthStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	monthlySummaries, err := getMonthlySummary(year, month)
	if err != nil {
		http.Error(w, "Failed to fetch monthly summary", http.StatusInternalServerError)
		return
	}

	// パート従業員の打刻データを取得
	partEmployeeCalendars := make(map[string][]WorkRecord)
	for _, emp := range monthlySummaries {
		if emp.EmploymentType == "パート" {
			records, err := getTimeRecords(emp.EmpNumber, monthStr)
			if err != nil {
				http.Error(w, "Failed to fetch employee calendar", http.StatusInternalServerError)
				return
			}
			partEmployeeCalendars[emp.EmpNumber] = records
		}
	}

	f := excelize.NewFile()

	// ※ 月次勤怠レポートシートはそのまま
	monthlySheet := "月次勤怠レポート"
	f.NewSheet(monthlySheet)
	headers := []string{"従業員番号", "従業員名", "時給", "合計勤務時間(分)", "合計夜勤時間(分)", "出勤日数", "交通費往復", "交通費", "月給", "有給取得日数", "メモ"}
	for i, h := range headers {
		cell, _ := excelize.ColumnNumberToName(i + 1)
		cell = fmt.Sprintf("%s1", cell)
		f.SetCellValue(monthlySheet, cell, h)
	}
	rowIndex := 2
	for _, ms := range monthlySummaries {
		f.SetCellValue(monthlySheet, fmt.Sprintf("A%d", rowIndex), ms.EmpNumber)
		f.SetCellValue(monthlySheet, fmt.Sprintf("B%d", rowIndex), ms.EmpName)
		f.SetCellValue(monthlySheet, fmt.Sprintf("C%d", rowIndex), ms.HourlyWage)
		f.SetCellValue(monthlySheet, fmt.Sprintf("D%d", rowIndex), ms.TotalWorkMin)
		f.SetCellValue(monthlySheet, fmt.Sprintf("E%d", rowIndex), ms.TotalNightShiftMin)
		f.SetCellValue(monthlySheet, fmt.Sprintf("F%d", rowIndex), ms.AttendanceDays)
		f.SetCellValue(monthlySheet, fmt.Sprintf("G%d", rowIndex), ms.TransportationExpense)
		f.SetCellValue(monthlySheet, fmt.Sprintf("H%d", rowIndex), ms.TransportationExpense*ms.AttendanceDays)
		f.SetCellValue(monthlySheet, fmt.Sprintf("I%d", rowIndex), ms.MonthlySalary)
		f.SetCellValue(monthlySheet, fmt.Sprintf("J%d", rowIndex), ms.PaidVacationTaken)
		f.SetCellValue(monthlySheet, fmt.Sprintf("K%d", rowIndex), ms.Memo)
		rowIndex++
	}

	// --- パート従業員カレンダーシート（横配置：1列空けて10人ずつ） ---
	calendarSheet := "パート従業員カレンダー"
	f.NewSheet(calendarSheet)

	// 対象の「パート」従業員のみを抽出
	var partEmployees []MonthlySummary
	for _, ms := range monthlySummaries {
		if ms.EmploymentType == "パート" {
			partEmployees = append(partEmployees, ms)
		}
	}

	// 配置設定
	blocksPerRow := 10 // 1行に表示する従業員数
	blockWidth := 4    // 1人分のカレンダーは4列（「日付」「時間」「分」「合計時間（分）」）
	// 各ブロックは、1行目：従業員名、2行目：ヘッダー、(daysInMonth)行：各日の記録、1行：合計、さらに2行の余白＝daysInMonth+5行
	daysInMonth := getDaysInMonth(year, month)
	blockHeight := daysInMonth + 5

	// 各従業員のカレンダーブロックを横に配置する
	for index, ms := range partEmployees {
		rowBlock := index / blocksPerRow        // ブロック行番号
		colBlock := index % blocksPerRow        // 同一行内でのブロック位置
		startRow := rowBlock*blockHeight + 1    // ブロック開始行
		startCol := colBlock*(blockWidth+1) + 1 // ブロック開始列（各ブロックの間に1列空ける）

		// 従業員名をブロックの最上部に表示
		colName, _ := excelize.ColumnNumberToName(startCol)
		f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", colName, startRow), ms.EmpName)

		// ヘッダー行（startRow+1）を出力
		headerRow := startRow + 1
		col1, _ := excelize.ColumnNumberToName(startCol)
		col2, _ := excelize.ColumnNumberToName(startCol + 1)
		col3, _ := excelize.ColumnNumberToName(startCol + 2)
		col4, _ := excelize.ColumnNumberToName(startCol + 3)
		f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col1, headerRow), "日付")
		f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col2, headerRow), "時間")
		f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col3, headerRow), "分")
		f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col4, headerRow), "合計時間（分）")

		// 対象従業員の打刻記録を日付ごとに集計
		records := partEmployeeCalendars[ms.EmpNumber]
		dailyRecords := make(map[string][]WorkRecord)
		for _, rec := range records {
			dailyRecords[rec.TargetDate] = append(dailyRecords[rec.TargetDate], rec)
		}

		totalMinutes := 0
		// 各日の記録
		for day := 1; day <= daysInMonth; day++ {
			currentRow := headerRow + day
			dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
			workMinutes := 0
			if recs, exists := dailyRecords[dateStr]; exists {
				clockIn := ""
				clockOut := ""
				breakDuration := 0
				for _, rec := range recs {
					if rec.TargetType == "clock_in" {
						clockIn = rec.TargetTime
					} else if rec.TargetType == "clock_out" {
						clockOut = rec.TargetTime
					} else if rec.TargetType == "break_duration" && rec.BreakDuration != nil {
						breakDuration += *rec.BreakDuration
					}
				}
				workMinutes = calculateWorkMinutes(clockIn, clockOut) - breakDuration
				if workMinutes < 0 {
					workMinutes = 0
				}
			}
			totalMinutes += workMinutes

			f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col1, currentRow), fmt.Sprintf("%d月%d日", month, day))
			f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col2, currentRow), workMinutes/60)
			f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col3, currentRow), workMinutes%60)
			f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col4, currentRow), workMinutes)
		}

		// 合計行（ヘッダー行の直下＋日数＋1行目）
		totalRow := headerRow + daysInMonth + 1
		totalHours := totalMinutes / 60
		totalRemaining := totalMinutes % 60
		f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col1, totalRow), "合計")
		f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col2, totalRow), totalHours)
		f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col3, totalRow), totalRemaining)
		f.SetCellValue(calendarSheet, fmt.Sprintf("%s%d", col4, totalRow), totalMinutes)
	}

	f.DeleteSheet("Sheet1")
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=勤怠レポート_%s.xlsx", monthStr))
	w.WriteHeader(http.StatusOK)
	f.Write(w)
}

// 労働時間計算
func calculateWorkMinutes(clockIn, clockOut string) int {
	if clockIn == "" || clockOut == "" {
		return 0
	}
	inParts := strings.Split(clockIn, ":")
	outParts := strings.Split(clockOut, ":")
	inH, _ := strconv.Atoi(inParts[0])
	inM, _ := strconv.Atoi(inParts[1])
	outH, _ := strconv.Atoi(outParts[0])
	outM, _ := strconv.Atoi(outParts[1])
	return (outH*60 + outM) - (inH*60 + inM)
}

func main() {
	initDB()
	initializeSecretAnswer()

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
	http.HandleFunc("/api/secretReset", secretResetHandler)
	http.HandleFunc("/api/saveMonthlyMemo", saveMonthlyMemoHandler)
	http.HandleFunc("/api/exportExcel", exportExcelHandler)

	// その後、静的ファイルサーバーを登録する
	http.Handle("/", http.FileServer(http.Dir("./static")))

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
