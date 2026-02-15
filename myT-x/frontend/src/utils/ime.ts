/**
 * IME変換中のキーイベントを判定する。
 * event.keyCode は Web標準で deprecated だが、Windows IME (特に日本語IME) では
 * isComposing/key だけでは変換中を正しく検出できないケースがある為、
 * keyCode === 229 によるフォールバック判定が必要。
 * NOTE: 将来の WebView2 更新で keyCode が削除された場合、代替手段の検討が必要。
 */
export function isImeTransitionalEvent(event: KeyboardEvent): boolean {
  return event.isComposing || event.key === "Process" || event.keyCode === 229;
}
