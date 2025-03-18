/**
 * 従業員一覧を取得して表示
 */
function fetchEmployees() {
  fetch('/api/employees')
    .then(response => {
      if (!response.ok) {
        throw new Error('サーバーエラー: ' + response.status);
      }
      return response.json();
    })
    .then(data => {
      const tableBody = document.getElementById("employeeTableBody");
      tableBody.innerHTML = ""; // 既存データをクリア

      data.forEach(emp => {
        // 従業員番号が "ZZ" または "ZY" で始まる場合はスキップする
        if (emp.employee_number.startsWith("ZZ") || emp.employee_number.startsWith("ZY")) {
          return;
        }
        
        const row = document.createElement("tr");
        row.innerHTML = `
          <td><a href="view_detail.html?empNumber=${emp.employee_number}">${emp.employee_number}</a></td>
          <td>${emp.name}</td>
          <td>${emp.job}</td>
          <td>${emp.job_code}</td>
          <td>
            <button class="btn btn-danger btn-sm" onclick="handleDeleteEmployee('${emp.employee_number}')">
              削除
            </button>
          </td>
        `;
        tableBody.appendChild(row);
      });
    })
    .catch(error => console.error('従業員取得エラー:', error));
}

/**
 * 従業員を削除する
 * @param {string} employeeNumber - 削除する従業員の番号
 */
function handleDeleteEmployee(employeeNumber) {
  if (!confirm(`従業員番号 ${employeeNumber} を削除しますか？`)) {
    return;
  }

  fetch(`/api/deleteEmployee`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ employee_number: employeeNumber })
  })
  .then(response => response.text())
  .then(data => {
    alert(data);
    fetchEmployees(); // 削除後にリスト更新
  })
  .catch(error => console.error('削除エラー:', error));
}

/**
 * 打刻不整合一覧を取得して表示
 */
function fetchInconsistencies() {
  fetch('/api/inconsistencies')
    .then(response => {
      if (!response.ok) {
        throw new Error("不整合一覧の取得に失敗しました");
      }
      return response.json();
    })
    .then(data => {
      const tbody = document.getElementById("inconsistenciesTableBody");
      tbody.innerHTML = "";
      
      if (!data || data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="4" class="text-center">不整合な打刻はありません</td></tr>`;
      } else {
        // 従業員番号で昇順にソートする
        data.sort((a, b) => {
          if (a.employee_number < b.employee_number) return -1;
          if (a.employee_number > b.employee_number) return 1;
          return 0;
        });
        
        data.forEach(item => {
          // "T"以降を取り除いて日付部分のみを表示
          const dateOnly = item.date.split("T")[0];
          const tr = document.createElement("tr");
          tr.innerHTML = `
            <td>${item.employee_number}</td>
            <td>${item.employee_name}</td>
            <td>${dateOnly}</td>
            <td>${item.issues.join(", ")}</td>
          `;
          tbody.appendChild(tr);
        });
      }
    })
    .catch(error => {
      document.getElementById("inconsistenciesTableBody").innerHTML = 
        `<tr><td colspan="4" class="text-center">エラーが発生しました: ${error.message}</td></tr>`;
    });
}

/**
 * 職種セレクトボックスにオプションを設定
 */
function populateEmployeeJobSelect() {
  fetch('/api/jobTypes')
    .then(response => {
      if (!response.ok) {
        throw new Error('職種一覧の取得に失敗しました');
      }
      return response.json();
    })
    .then(data => {
      const select = document.getElementById('employee_job');
      // 既存のオプションをクリアし、初期オプションを設定
      select.innerHTML = '<option value="">-- 職種を選択してください --</option>';
      
      data.forEach(job => {
        const option = document.createElement('option');
        option.value = job.name;
        option.textContent = `${job.name} (${job.code})`;
        select.appendChild(option);
      });
    })
    .catch(error => {
      console.error('職種取得エラー:', error);
    });
}

/**
 * 従業員の追加処理
 * @param {Event} event - フォーム送信イベント
 */
function handleAddEmployee(event) {
  event.preventDefault();
  
  const record = {
    name: document.getElementById("employee_name").value,
    job: document.getElementById("employee_job").value,
    max_attendance_count: 20,
    paid_vacation_limit: parseInt(document.getElementById("paid_vacation_limit").value, 10),
    paid_vacation_grant_date: (function() {
      const joiningDateStr = document.getElementById("paid_vacation_grant_date").value;
      const joiningDate = new Date(joiningDateStr);
      joiningDate.setMonth(joiningDate.getMonth() + 6);
      return joiningDate.toISOString().split("T")[0];
    })(),
    employment_type: document.querySelector('input[name="employment_type"]:checked').value,
    hourly_wage: 0,
    transportation_expense: 0
  };
  
  if (record.employment_type === "パート") {
    record.hourly_wage = parseInt(document.getElementById("hourly_wage").value, 10) || 0;
    record.transportation_expense = parseInt(document.getElementById("transportation_expense").value, 10) || 0;
  }
  
  fetch('/api/addEmployee', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(record)
  })
  .then(response => {
    if (!response.ok) {
      return response.text().then(text => { throw new Error(text); });
    }
    return response.json();
  })
  .then(data => {
    alert('従業員を追加しました: ' + data.employee_number + ' ' + data.name);
    fetchEmployees();
    document.getElementById("addEmployeeForm").reset();
  })
  .catch(error => {
    alert('エラー: ' + error.message);
  });
}

/**
 * 職種一覧を取得して表示
 */
function fetchJobTypes() {
  fetch('/api/jobTypes')
    .then(response => response.json())
    .then(data => {
      // 職種一覧テーブルの更新
      const tableBody = document.getElementById("jobTypeTableBody");
      tableBody.innerHTML = ""; // 既存データをクリア

      data.forEach(job => {
        const row = document.createElement("tr");
        // 職種名部分を span でラップし、data-code属性に職種コードを保持
        row.innerHTML = `
          <td>${job.code}</td>
          <td><span class="editable-job-name" data-code="${job.code}">${job.name}</span></td>
          <td>
            <button class="btn btn-danger btn-sm" onclick="handleDeleteJobType('${job.code}')">
              削除
            </button>
          </td>
        `;
        tableBody.appendChild(row);
      });

      // span要素に編集イベントを付与
      document.querySelectorAll('.editable-job-name').forEach(span => {
        span.addEventListener('click', function() {
          // 既にinputが存在していなければ編集モードにする
          if (this.querySelector('input')) return;
          
          const currentName = this.innerText;
          const code = this.dataset.code;
          const input = document.createElement('input');
          input.type = 'text';
          input.value = currentName;
          input.className = 'form-control';
          input.style.width = '80%';

          // 入力完了（Enterキーまたはblur）時に更新
          function finishEditing() {
            const newName = input.value.trim();
            if (newName && newName !== currentName) {
              // APIに更新リクエストを送信
              fetch('/api/updateJobType', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ code: code, name: newName })
              })
              .then(response => {
                if (!response.ok) {
                  throw new Error('更新に失敗しました');
                }
                return response.json();
              })
              .then(data => {
                span.innerText = data.name;
              })
              .catch(error => {
                alert(error.message);
                span.innerText = currentName;
              });
            } else {
              span.innerText = currentName;
            }
          }

          input.addEventListener('keydown', function(e) {
            if (e.key === 'Enter') {
              finishEditing();
            }
          });
          input.addEventListener('blur', finishEditing);

          // spanの中身をinputに置換
          span.innerHTML = '';
          span.appendChild(input);
          input.focus();
        });
      });
    })
    .catch(error => console.error('職種取得エラー:', error));
}

/**
 * 職種を削除する
 * @param {string} jobCode - 削除する職種コード
 */
function handleDeleteJobType(jobCode) {
  if (!confirm(`職種コード ${jobCode} を削除しますか？`)) {
    return;
  }

  fetch(`/api/deleteJobType`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code: jobCode })
  })
  .then(response => response.text())
  .then(data => {
    alert(data);
    fetchJobTypes(); // 削除後にリスト更新
  })
  .catch(error => console.error('削除エラー:', error));
}

/**
 * 新しい職種を追加する
 * @param {Event} event - フォーム送信イベント
 */
function handleAddJobType(event) {
  event.preventDefault();

  const record = {
    code: document.getElementById("job_type_code").value.trim(),
    name: document.getElementById("job_type_name").value.trim()
  };

  fetch('/api/addJobType', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(record)
  })
  .then(response => response.text())
  .then(data => {
    alert(data);
    fetchJobTypes(); // 更新
    document.getElementById("addJobTypeForm").reset();
  })
  .catch(error => console.error('エラー:', error));
}

/**
 * パスワード設定の処理
 * @param {Event} event - フォーム送信イベント
 * @returns {boolean} - フォーム送信の処理結果
 */
function handleSetOwner(event) {
  event.preventDefault();
  
  const newPassword = document.getElementById("new_password").value;
  const confirmPassword = document.getElementById("confirm_password").value;
  
  if (newPassword !== confirmPassword) {
    alert("新しいパスワードが一致しません。");
    return false;
  }
  
  // hidden フィールドから既存のメールアドレスを取得
  const email = document.getElementById("owner_email_hidden").value;
  const payload = {
    email: email,
    password: newPassword
  };

  fetch('/api/setOwner', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  })
  .then(response => {
    if (!response.ok) {
      throw new Error('サーバーエラー: ' + response.status);
    }
    return response.text();
  })
  .then(data => {
    document.getElementById("owner_message").innerText = data;
    document.getElementById("owner_message").className = "mt-2 text-success";
    document.getElementById("ownerForm").reset();
  })
  .catch(error => {
    document.getElementById("owner_message").innerText = error.message;
    document.getElementById("owner_message").className = "mt-2 text-danger";
  });
  
  return false;
}

// ページ読み込み時の処理
document.addEventListener('DOMContentLoaded', function() {
  // 従業員と職種のデータを取得
  fetchEmployees();
  fetchInconsistencies();
  populateEmployeeJobSelect();
  fetchJobTypes();
  
  // パート/正社員の切り替えイベント設定
  document.querySelectorAll('input[name="employment_type"]').forEach(radio => {
    radio.addEventListener('change', function() {
      const parttimeFields = document.getElementById('parttimeFields');
      if (this.value === 'パート') {
        parttimeFields.style.display = 'block';
      } else {
        parttimeFields.style.display = 'none';
      }
    });
  });
  
  // 所有者情報を取得
  fetch('/api/getOwner')
    .then(response => response.json())
    .then(data => {
      document.getElementById("owner_email_hidden").value = data.email || "";
    })
    .catch(error => console.error('オーナー情報取得エラー:', error));
});