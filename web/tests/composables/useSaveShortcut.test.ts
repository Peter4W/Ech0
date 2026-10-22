// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2025-2026 lin-snow

import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { useSaveShortcut } from '@/composables/useSaveShortcut'

type HostOptions = {
  active?: boolean
  busy?: boolean
}

const wrappers: VueWrapper[] = []

const mountHost = (options: HostOptions = {}) => {
  const active = ref(options.active ?? true)
  const busy = ref(options.busy ?? false)
  const onSave = vi.fn()
  const wrapper = mount(
    defineComponent({
      setup() {
        useSaveShortcut({ active: () => active.value, busy: () => busy.value, onSave })
        return () => h('div')
      },
    }),
  )
  wrappers.push(wrapper)
  return { wrapper, active, busy, onSave }
}

const pressSave = (init: KeyboardEventInit = {}) => {
  const event = new KeyboardEvent('keydown', {
    key: 's',
    ctrlKey: true,
    cancelable: true,
    ...init,
  })
  window.dispatchEvent(event)
  return event
}

afterEach(() => {
  wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
})

describe('useSaveShortcut', () => {
  it('saves and suppresses the browser dialog while active', () => {
    const { onSave } = mountHost()

    const event = pressSave()

    expect(onSave).toHaveBeenCalledTimes(1)
    expect(event.defaultPrevented).toBe(true)
  })

  it('follows the active getter', () => {
    const { active, onSave } = mountHost()

    active.value = false
    const event = pressSave()
    expect(onSave).not.toHaveBeenCalled()
    expect(event.defaultPrevented).toBe(false)

    active.value = true
    pressSave()
    expect(onSave).toHaveBeenCalledTimes(1)
  })

  it('ignores extra modifiers', () => {
    const { onSave } = mountHost()

    pressSave({ altKey: true })
    pressSave({ ctrlKey: true, shiftKey: true })

    expect(onSave).not.toHaveBeenCalled()
  })

  it('ignores key repeat and IME composition but still blocks the dialog', () => {
    const { onSave } = mountHost()

    const repeated = pressSave({ repeat: true })
    const composing = pressSave({ isComposing: true })

    expect(onSave).not.toHaveBeenCalled()
    expect(repeated.defaultPrevented).toBe(true)
    expect(composing.defaultPrevented).toBe(true)
  })

  it('does nothing while busy', () => {
    const { busy, onSave } = mountHost()

    busy.value = true
    pressSave()

    expect(onSave).not.toHaveBeenCalled()
  })

  it('ignores events already handled by another listener', () => {
    const { onSave } = mountHost()

    const event = new KeyboardEvent('keydown', {
      key: 's',
      ctrlKey: true,
      cancelable: true,
    })
    event.preventDefault()
    window.dispatchEvent(event)

    expect(onSave).not.toHaveBeenCalled()
  })

  it('matches the physical S key on non-Latin layouts', () => {
    const { onSave } = mountHost()

    pressSave({ key: 'o', code: 'KeyS' })

    expect(onSave).toHaveBeenCalledTimes(1)
  })

  it('removes the listener on unmount', () => {
    const { wrapper, onSave } = mountHost()

    wrapper.unmount()
    pressSave()

    expect(onSave).not.toHaveBeenCalled()
  })
})
