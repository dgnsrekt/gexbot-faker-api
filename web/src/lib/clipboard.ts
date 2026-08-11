// copyText copies text to the clipboard, returning whether it succeeded.
//
// The async Clipboard API (navigator.clipboard) only exists in a *secure
// context* — HTTPS or localhost. The Studio is commonly served over plain HTTP
// on a LAN host (e.g. http://llm2-studio.local:8080), where navigator.clipboard
// is undefined, so we fall back to a hidden-textarea + execCommand('copy'),
// which still works in insecure contexts.
export async function copyText(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // fall through to the legacy path
    }
  }
  // Preserve focus: selecting the temporary textarea steals it from the button,
  // and removing the textarea would otherwise drop focus to <body>, disorienting
  // keyboard users. Teardown lives in finally so the textarea is always removed
  // and focus restored even if execCommand throws.
  const prevFocus = document.activeElement as HTMLElement | null
  const ta = document.createElement('textarea')
  try {
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.top = '-1000px'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    ta.remove()
    prevFocus?.focus?.()
  }
}
