// 従業員リストの取得と表示
function fetchEmployees() {
    fetch('/api/employees')
      .then(response => {
        if (!response.ok) {
          throw new Error('サーバーエラー: ' + response.status);
        }
        return response.json();
      })
      .then(data => {
        const select = document.getElementById("employeeSelect");
        select.innerHTML = '<option value="">-- 従業員を選択してください --</option>';
        data.forEach(emp => {
          const option = document.createElement("option");
          option.value = emp.id;
          option.text = `${emp.name} (${emp.job})`;
          select.add(option);
        });
      })
      .catch(error => console.error('従業員取得エラー:', error));
  }
  
  // 日本時間の日付文字列を取得する関数
  function getJapanDateString() {
    const now = new Date();
    now.setMinutes(now.getMinutes() - now.getTimezoneOffset()); // UTCからJSTへ変換
    const year = now.getFullYear();
    const month = (now.getMonth() + 1).toString().padStart(2, "0"); // 2桁にする
    const day = now.getDate().toString().padStart(2, "0"); // 2桁にする
    return `${year}-${month}-${day}`; // YYYY-MM-DD 形式
  }
  
  // タイムレコーダーの記録を保存する関数
function recordImmediate(type) {
  const empSelect = document.getElementById("employeeSelect");
  if (empSelect.value === "") {
    alert("従業員を選択してください");
    return;
  }

  const now = new Date();
  const target_date = getJapanDateString(); // YYYY-MM-DD 形式
  const target_time = now.toTimeString().slice(0, 5); // HH:MM 形式

  const record = {
    employee_id: parseInt(empSelect.value, 10),
    target_date: target_date,
    target_time: target_time,
    target_type: type
  };

  // 休憩時間登録の場合、入力値をペイロードに追加
  if (type === 'break_duration') {
    const breakDurationValue = document.getElementById("breakDurationInput").value;
    const breakDuration = parseInt(breakDurationValue, 10);
    if (isNaN(breakDuration) || breakDuration < 0) {
      alert("休憩時間は0以上の値を正しく入力してください");
      return;
    }
    record.break_duration = breakDuration;
  }

  fetch('/api/saveWorkRecord', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(record)
  })
  .then(response => {
    if (!response.ok) {
      throw new Error('サーバーエラー: ' + response.status);
    }
    return response.text();
  })
  .then(data => {
    // 成功時のメッセージを操作タイプに応じて変更
    let successMessage;
    switch(type) {
      case 'clock_in':
        successMessage = "出勤の打刻に成功しました";
        break;
      case 'clock_out':
        successMessage = "退勤の打刻に成功しました";
        break;
      case 'break_duration':
        successMessage = "休憩時間の登録に成功しました";
        break;
      default:
        successMessage = data;
    }
    alert(successMessage);
    
    // 休憩時間入力欄をクリア
    if(type === 'break_duration') {
      document.getElementById("breakDurationInput").value = "";
    }
  })
  .catch(error => {
    alert("エラー: " + error.message);
    console.error('エラー:', error);
  });
}
  
  // 有給休暇の登録処理
    function handlePaidVacationSubmit(formObject) {
        const empSelect = document.getElementById("employeeSelect");
        if (empSelect.value === "") {
        alert("従業員を選択してください");
        return false;
        }

        const record = {
        employee_id: parseInt(empSelect.value, 10),
        target_date: formObject.target_date.value,
        target_time: "00:00",
        target_type: "paid_vacation"
        };

        fetch('/api/saveWorkRecord', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(record)
        })
        .then(response => {
        if (!response.ok) {
            // エラーレスポンスの内容を取得して処理する
            return response.text().then(errorText => {
            throw new Error(errorText || `サーバーエラー: ${response.status}`);
            });
        }
        return response.text();
        })
        .then(data => {
        alert(data);
        formObject.reset(); // フォームをリセット
        })
        .catch(error => {
        // エラーメッセージを確認して、有給不足の場合は専用メッセージを表示
        const errorMsg = error.message;
        if (errorMsg.includes("有給休暇が不足") || 
            errorMsg.includes("使用できる有給休暇がありません") ||
            errorMsg.includes("available") || 
            errorMsg.includes("insufficient")) {
            alert("取得可能な有給休暇日数がありません。");
        } else {
            // その他のエラーの場合は元のエラーメッセージを表示
            alert("エラー: " + errorMsg);
        }
        console.error('エラー:', error);
        });

        return false;
    }
  
  // ページ読み込み時に従業員リストを取得
  document.addEventListener('DOMContentLoaded', function() {
    fetchEmployees();
  });