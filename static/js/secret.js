/**
 * パスワード再設定フォームの送信処理
 * @param {Event} e - フォーム送信イベント
 */
function handleSecretReset(e) {
    e.preventDefault();
    const newPassword = document.getElementById("new_password").value;
    const confirmPassword = document.getElementById("confirm_password").value;
    const secretAnswer = document.getElementById("secret_answer").value.trim();
  
    if (newPassword !== confirmPassword) {
      setMessage("パスワードが一致しません。", "text-danger");
      return;
    }
  
    fetch("/api/secretReset", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        new_password: newPassword,
        secret_answer: secretAnswer
      })
    })
    .then(response => {
      if (!response.ok) {
        return response.text().then(text => { throw new Error(text); });
      }
      return response.text();
    })
    .then(data => {
      // 成功メッセージを表示
      setMessage(data, "text-success");
      // フォームをクリア
      document.getElementById("secretResetForm").reset();
    })
    .catch(error => {
      setMessage(error.message, "text-danger");
    });
  }
  
  /**
   * メッセージを表示する関数
   * @param {string} message - 表示するメッセージ
   * @param {string} className - 適用するBootstrapクラス
   */
  function setMessage(message, className = "") {
    const messageElement = document.getElementById("message");
    messageElement.innerText = message;
    
    // すべてのスタイルクラスを削除
    messageElement.classList.remove("text-danger", "text-success", "text-warning");
    
    // 新しいクラスを追加（指定されている場合）
    if (className) {
      messageElement.classList.add(className);
    }
  }
  
  // ページ読み込み時の処理
  document.addEventListener("DOMContentLoaded", function() {
    const form = document.getElementById("secretResetForm");
    if (form) {
      form.addEventListener("submit", handleSecretReset);
    }
  });