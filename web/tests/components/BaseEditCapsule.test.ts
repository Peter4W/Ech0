// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2025-2026 lin-snow

import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import BaseEditCapsule from '@/components/common/BaseEditCapsule.vue'

type CapsuleProps = {
  editing?: boolean
  saveShortcut?: boolean
  busy?: boolean
}

const wrappers: VueWrapper[] = []

const mountCapsule = (props: CapsuleProps = {}) => {
  const wrapper = mount(BaseEditCapsule, {
    props: { editing: true, ...props },
    global: { directives: { tooltip: {} } },
  })
  wrappers.push(wrapper)
  return wrapper
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

describe('BaseEditCapsule save shortcut', () => {
  it('emits apply and suppresses the browser save dialog while editing', () => {
    const wrapper = mountCapsule()

    const event = pressSave()

    expect(event.defaultPrevented).toBe(true)
    expect(wrapper.emitted('apply')).toHaveLength(1)
  })

  it('stays inert when not editing', () => {
    const wrapper = mountCapsule({ editing: false })

    const event = pressSave()

    expect(event.defaultPrevented).toBe(false)
    expect(wrapper.emitted('apply')).toBeUndefined()
  })

  it('supports Cmd+S on macOS', () => {
    const wrapper = mountCapsule()

    pressSave({ ctrlKey: false, metaKey: true })

    expect(wrapper.emitted('apply')).toHaveLength(1)
  })

  it('ignores extra modifiers, key repeat and IME composition', () => {
    const wrapper = mountCapsule()

    pressSave({ altKey: true })
    pressSave({ ctrlKey: true, shiftKey: true })
    pressSave({ repeat: true })
    pressSave({ isComposing: true })

    expect(wrapper.emitted('apply')).toBeUndefined()
  })

  it('does not run while a save is in flight', () => {
    const wrapper = mountCapsule({ busy: true })

    const event = pressSave()

    expect(event.defaultPrevented).toBe(true)
    expect(wrapper.emitted('apply')).toBeUndefined()
  })

  it('can opt out and no longer advertises the shortcut', () => {
    const wrapper = mountCapsule({ saveShortcut: false })

    pressSave()

    expect(wrapper.emitted('apply')).toBeUndefined()
    expect(wrapper.get('button').attributes('aria-keyshortcuts')).toBeUndefined()
  })

  it('advertises the shortcut on the apply button', () => {
    const wrapper = mountCapsule()

    expect(wrapper.get('button').attributes('aria-keyshortcuts')).toBe('Control+S Meta+S')
    expect(wrapper.get('button').attributes('aria-label')).toMatch(/\((⌘S|Ctrl\+S)\)$/)
  })
})
