// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2025-2026 lin-snow

import { onBeforeUnmount, onMounted } from 'vue'

type SaveShortcutOptions = {
  /** Shortcut only reacts while this returns true (edit mode, open form, …). */
  active: () => boolean
  /** Skip while a save is already in flight. */
  busy?: () => boolean
  /** Runs on a clean Ctrl+S / Cmd+S press. */
  onSave: () => void
}

/**
 * Binds Ctrl+S / Cmd+S to an in-app save action while the caller is active.
 * The browser's own "Save page" dialog is suppressed only for an active save.
 */
export const useSaveShortcut = ({ active, busy, onSave }: SaveShortcutOptions) => {
  const handleKeydown = (event: KeyboardEvent) => {
    const isSaveKey =
      (event.ctrlKey || event.metaKey) &&
      !event.altKey &&
      !event.shiftKey &&
      (event.key.toLowerCase() === 's' || event.code === 'KeyS')

    if (!isSaveKey || event.defaultPrevented || !active()) return

    event.preventDefault()
    // A composition that has not reached v-model would save a stale value.
    if (event.isComposing || event.repeat || busy?.()) return

    onSave()
  }

  onMounted(() => window.addEventListener('keydown', handleKeydown))
  onBeforeUnmount(() => window.removeEventListener('keydown', handleKeydown))
}
