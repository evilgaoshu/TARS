export const COMMAND_HUB_TOGGLE_EVENT = 'tars:command-hub:toggle'
export const COMMAND_HUB_OPEN_EVENT = 'tars:command-hub:open'

export function toggleCommandHub() {
  window.dispatchEvent(new CustomEvent(COMMAND_HUB_TOGGLE_EVENT))
}

export function openCommandHub() {
  window.dispatchEvent(new CustomEvent(COMMAND_HUB_OPEN_EVENT))
}
