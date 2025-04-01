
# MySQL テーブル構造とデータ出力

## `show tables;`

```
+-----------------------+
| Tables_in_attendance  |
+-----------------------+
| employees             |
| job_types             |
| monthly_memos         |
| paid_vacation_history |
| system_config         |
| work_records          |
+-----------------------+
6 rows in set (0.00 sec)
```

---

## `describe employees;`

| Field                    | Type         | Null | Key | Default   | Extra          |
|--------------------------|--------------|------|-----|-----------|----------------|
| id                       | int          | NO   | PRI | NULL      | auto_increment |
| name                     | varchar(100) | NO   |     | NULL      |                |
| job                      | varchar(100) | NO   |     | NULL      |                |
| max_attendance_count     | int          | NO   |     | 20        |                |
| paid_vacation_limit      | int          | YES  |     | NULL      |                |
| paid_vacation_grant_date | date         | YES  |     | NULL      |                |
| job_code                 | char(2)      | NO   | MUL | NULL      |                |
| employee_number          | char(8)      | YES  | UNI | NULL      |                |
| employment_type          | varchar(10)  | NO   |     | 正社員    |                |
| hourly_wage              | int          | YES  |     | 0         |                |
| transportation_expense   | int          | YES  |     | 0         |                |

---

## `describe job_types;`

| Field | Type         | Null | Key | Default | Extra |
|-------|--------------|------|-----|---------|-------|
| code  | char(2)      | NO   | PRI | NULL    |       |
| name  | varchar(100) | NO   |     | NULL    |       |

---

## `describe monthly_memos;`

| Field       | Type      | Null | Key | Default           | Extra                                         |
|-------------|-----------|------|-----|-------------------|-----------------------------------------------|
| id          | int       | NO   | PRI | NULL              | auto_increment                                |
| employee_id | int       | NO   | MUL | NULL              |                                               |
| month       | char(7)   | NO   |     | NULL              |                                               |
| memo        | text      | YES  |     | NULL              |                                               |
| updated_at  | timestamp | YES  |     | CURRENT_TIMESTAMP | DEFAULT_GENERATED on update CURRENT_TIMESTAMP |

---

## `describe paid_vacation_history;`

| Field            | Type     | Null | Key | Default           | Extra             |
|------------------|----------|------|-----|-------------------|-------------------|
| id               | int      | NO   | PRI | NULL              | auto_increment    |
| employee_id      | int      | NO   | MUL | NULL              |                   |
| grant_date       | date     | NO   |     | NULL              |                   |
| granted_days     | int      | NO   |     | NULL              |                   |
| record_timestamp | datetime | NO   |     | CURRENT_TIMESTAMP | DEFAULT_GENERATED |

---

## `describe system_config;`

| Field              | Type         | Null | Key | Default | Extra |
|--------------------|--------------|------|-----|---------|-------|
| id                 | int          | NO   | PRI | NULL    |       |
| owner_email        | varchar(255) | YES  |     | NULL    |       |
| owner_password     | varchar(255) | YES  |     | NULL    |       |
| secret_answer      | varchar(255) | NO   |     |         |       |
| secret_answer_hash | varchar(255) | NO   |     |         |       |

---

## `describe work_records;`

| Field          | Type                                                                                    | Null | Key | Default | Extra          |
|----------------|-----------------------------------------------------------------------------------------|------|-----|---------|----------------|
| id             | int                                                                                     | NO   | PRI | NULL    | auto_increment |
| employee_id    | int                                                                                     | NO   | MUL | NULL    |                |
| target_date    | date                                                                                    | NO   |     | NULL    |                |
| target_time    | time                                                                                    | NO   |     | NULL    |                |
| target_type    | enum('clock_in','clock_out','paid_vacation','break_start','break_end','break_duration') | NO   |     | NULL    |                |
| memo           | varchar(255)                                                                            | YES  |     |         |                |
| break_duration | int                                                                                     | YES  |     | NULL    |                |

---

## `select * from system_config;`


| id | owner_email            | owner_password                                               | secret_answer | secret_answer_hash                                           |
|----|------------------------|--------------------------------------------------------------|----------------|--------------------------------------------------------------|
| 1  | nyaruko65005@gmail.com | $2a$10$1miiBUhjNVtgFXHzYt3daeeyZe9oL0ESOv7NB7uAt6Gmi2QYS9e6y | stroll         | $2a$10$JE006Idg4n5TJ5Us7Yqqou.fK8HSdHT4N8ljv04TDpE2HwKKewcty |

