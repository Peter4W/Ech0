// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2025-2026 lin-snow

const platformHints =
  typeof navigator === 'undefined' ? '' : `${navigator.platform} ${navigator.userAgent}`

/** iPadOS reports itself as a Mac in desktop mode, so check both hints. */
export const isMac = /mac|iphone|ipad|ipod/i.test(platformHints)
