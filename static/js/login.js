/**
 * ログイン処理を行う関数
 * @param {Event} e - フォーム送信イベント
 */
function handleLogin(e) {
    e.preventDefault();
    const password = document.getElementById("adminPassword").value;
  
    fetch("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password: password })
    })
    .then(response => {
      if (!response.ok) {
        // 401など → パスワード不一致
        throw new Error("ログインに失敗しました");
      }
      return response.text();
    })
    .then(data => {
      // 成功 → view_home.html にリダイレクト
      window.location.href = "view_home.html";
    })
    .catch(error => {
      // 失敗 → エラーメッセージを表示
      document.getElementById("loginMessage").innerText = error.message;
    });
  }
  
  /**
   * ページ読み込み時の処理
   */
  document.addEventListener('DOMContentLoaded', function() {
    // ログインフォームにイベントリスナーを設定
    const loginForm = document.getElementById('loginForm');
    if (loginForm) {
      loginForm.addEventListener('submit', handleLogin);
    }
  });