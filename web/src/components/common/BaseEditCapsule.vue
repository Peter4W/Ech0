<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Copyright (C) 2025-2026 lin-snow -->
<template>
  <div
    class="inline-flex items-center rounded-full ring-1 ring-inset ring-[var(--color-border-subtle)] bg-[var(--input-bg-color)] overflow-hidden"
  >
    <button
      v-if="editing"
      type="button"
      v-tooltip="applyTooltip"
      :aria-label="applyTooltip"
      :aria-keyshortcuts="saveShortcut ? 'Control+S Meta+S' : undefined"
      class="w-7 h-7 flex items-center justify-center text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-border-subtle)] transition-colors duration-200"
      @click="$emit('apply')"
    >
      <Saveupdate class="w-4 h-4" />
    </button>
    <button
      type="button"
      v-tooltip="editing ? cancelTitle : editTitle"
      :aria-label="editing ? cancelTitle : editTitle"
      class="w-7 h-7 flex items-center justify-center text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-border-subtle)] transition-colors duration-200"
      @click="handleSecondaryClick"
    >
      <Close v-if="editing" class="w-4 h-4" />
      <Edit v-else class="w-4 h-4" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Edit from '@/components/icons/edit.vue'
import Close from '@/components/icons/close.vue'
import Saveupdate from '@/components/icons/saveupdate.vue'
import { useSaveShortcut } from '@/composables/useSaveShortcut'
import { isMac } from '@/utils/platform'

const props = withDefaults(
  defineProps<{
    editing: boolean
    editTitle?: string
    cancelTitle?: string
    applyTitle?: string
    saveShortcut?: boolean
    busy?: boolean
  }>(),
  {
    editTitle: '编辑',
    cancelTitle: '取消',
    applyTitle: '应用',
    saveShortcut: true,
    busy: false,
  },
)

const emit = defineEmits<{
  apply: []
  toggle: []
}>()

const saveShortcutLabel = isMac ? '⌘S' : 'Ctrl+S'
const applyTooltip = computed(() =>
  props.saveShortcut ? `${props.applyTitle} (${saveShortcutLabel})` : props.applyTitle,
)

useSaveShortcut({
  active: () => props.editing && props.saveShortcut,
  busy: () => props.busy,
  onSave: () => emit('apply'),
})

const handleSecondaryClick = () => {
  emit('toggle')
}
</script>
