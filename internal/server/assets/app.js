"use strict";

let copied = false;
let submitted = false;

// heartbeat: サーバは最後のビートから timeout でシャットダウンする。
// 最初の 1 拍はロード直後に打つ: サーバは初回ビートを見るまで時計を持たないので、
// 5 秒以内に閉じられたタブがゾンビサーバを残さないようにする(ゾンビ厳禁ルール)。
function heartbeat() {
  fetch("/api/heartbeat", { method: "POST" }).catch(() => {});
}
heartbeat();
setInterval(heartbeat, 5000);
// 背景タブの setInterval はブラウザに間引かれる(Chrome は数分後に約 1 分粒度)。
// option の links は _blank で開くので背景化は想定内の動線であり、復帰した瞬間に
// 1 拍打ってタイムアウト経過での誤シャットダウンを防ぐ。
document.addEventListener("visibilitychange", () => {
  if (!document.hidden) heartbeat();
});

// showError は失敗を画面に出す。JS が黙って死ぬと、人間には
// 「ボタンが効かない」としか見えない。
function showError(id, message) {
  const el = document.getElementById(id);
  el.textContent = message;
  el.hidden = message === "";
}

// collectAnswers は「触られた質問」だけを載せる。空文字・空配列・空の other は
// 送らない(サーバ側も未回答として扱うが、payload は発生源で綺麗にしておく)。
function collectAnswers() {
  const answers = {};
  for (const fs of document.querySelectorAll("fieldset[data-question]")) {
    const id = fs.dataset.question;
    const type = fs.dataset.type;
    const other = fs.querySelector("[data-role=other]");
    const otherValue = other ? other.value.trim() : "";
    if (type === "free_text") {
      const v = fs.querySelector("textarea").value.trim();
      if (v) answers[id] = v;
    } else if (type === "yes_no") {
      const sel = fs.querySelector("input:checked");
      if (sel) answers[id] = sel.value === "yes";
    } else if (type === "single_choice") {
      const sel = fs.querySelector("input[type=radio]:checked");
      if (otherValue) answers[id] = otherValue; // 自由入力があればそれが答え
      else if (sel) answers[id] = sel.value;
    } else if (type === "multi_choice") {
      const vals = [...fs.querySelectorAll("input[type=checkbox]:checked")].map((i) => i.value);
      if (otherValue) vals.push(otherValue);
      if (vals.length) answers[id] = vals;
    }
  }
  return answers;
}

document.getElementById("answer-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  // 前回の結果は、まず畳んでから送る。畳まないと、失敗した再提出のあとに
  // 「古いクリップボードテキスト + Copied ✓」が残り、人間は新しい回答を
  // コピーしたつもりで古い方を貼ってしまう(黙って間違える一番の経路)。
  showError("submit-error", "");
  showError("result-error", "");
  document.getElementById("copy-button").textContent = "Copy to clipboard";
  document.getElementById("result").hidden = true;
  try {
    const res = await fetch("/api/answers", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ answers: collectAnswers() }),
    });
    // エラー応答は http.Error のプレーンテキストなので、json() の前に弾く。
    if (!res.ok) throw new Error(`${res.status} ${(await res.text()).trim()}`);
    const { clipboard } = await res.json();
    submitted = true;
    copied = false; // 再提出したらテキストが変わるので、コピーはやり直し
    document.getElementById("clipboard-preview").textContent = clipboard;
    const result = document.getElementById("result");
    result.hidden = false;
    result.scrollIntoView({ behavior: "smooth" });
  } catch (err) {
    // fetch がネットワークレベルで失敗したときだけ TypeError になる(応答が 1 つも
    // 返ってこなかった = サーバがもういない)。この状態で「押し直して」と言うのは
    // 嘘なので、手で拾って再実行を頼む導線に切り替える。それ以外(4xx/5xx や
    // 壊れた JSON)はサーバが生きている証拠なので、従来どおり再送を促す。
    if (err instanceof TypeError) {
      showError("submit-error", "Could not reach optioner — this session appears to have ended (the server is gone). Your answers are still on this page: copy them by hand, and ask the agent to run optioner again.");
    } else {
      // 回答はフォームに残っている。人間が Submit を押し直せる状態にして戻す。
      showError("submit-error", `Could not submit your answers (${err.message}). Your answers are still on this page — press Submit again.`);
    }
  }
});

document.getElementById("copy-button").addEventListener("click", async () => {
  const text = document.getElementById("clipboard-preview").textContent;
  try {
    await navigator.clipboard.writeText(text);
  } catch (err) {
    showError("result-error", `Could not copy to the clipboard (${err.message}). Select the text above and copy it by hand.`);
    return;
  }
  showError("result-error", "");
  copied = true;
  document.getElementById("copy-button").textContent = "Copied ✓";
});

document.getElementById("done-button").addEventListener("click", async () => {
  let closedWithoutCopy = false;
  if (!copied) {
    try {
      const text = document.getElementById("clipboard-preview").textContent;
      await navigator.clipboard.writeText(text); // Done は暗黙でコピーもする
      copied = true;
    } catch {
      // コピーできなくても close は必ず打つ: 押しても何も起きない Done は
      // プロセスを人質に取る(ゾンビ厳禁ルール)。代わりにページは残し、
      // 本文を手で拾えるようにする。
      closedWithoutCopy = true;
    }
  }
  await fetch("/api/close", { method: "POST" }).catch(() => {});
  if (closedWithoutCopy) {
    showError("result-error", "Optioner has shut down, but the text could not be copied automatically. Use the Copy button, or select the text above and copy it by hand.");
    return;
  }
  document.body.innerHTML = "<main><h1>Optioner is shut down. You can close this tab.</h1></main>";
});

// 回答済み・未コピーのままタブを閉じようとしたら標準の確認を出す
// (文言はブラウザ仕様で固定。「コピーしますか?」はページ内の Done フローが担う)
window.addEventListener("beforeunload", (e) => {
  if (submitted && !copied) e.preventDefault();
});
