export function redirectToOAuth(url: string): void {
  let parsed: URL
  try {
    parsed = new URL(url)
  } catch {
    throw new Error('Received an invalid authorization URL')
  }

  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw new Error('Received an unsafe authorization URL protocol')
  }

  window.location.assign(parsed.toString())
}
