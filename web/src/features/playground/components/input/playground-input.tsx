/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  PromptInput,
  PromptInputFooter,
  PromptInputTextarea,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'

import { useEnabledPlugins } from '../../hooks/use-enabled-plugins'
import { getSubmittableInputText } from '../../lib'
import type {
  ModelOption,
  GroupOption,
  ParameterEnabled,
  PlaygroundConfig,
} from '../../types'
import { PlaygroundInputControls } from './playground-input-controls'
import { PlaygroundInputTools } from './playground-input-tools'
import { filterPluginMentions } from './plugin-mention-utils'
import { PluginMentionDropdown } from './plugin-mention-dropdown'

// Keys that move the caret without changing the text; used to decide when a
// keyup should trigger a mention rescan.
const CARET_MOVE_KEYS = new Set([
  'ArrowLeft',
  'ArrowRight',
  'Home',
  'End',
  'PageUp',
  'PageDown',
])

interface PlaygroundInputProps {
  config: PlaygroundConfig
  onSubmit: (text: string) => void
  onStop?: () => void
  disabled?: boolean
  isGenerating?: boolean
  models: ModelOption[]
  modelValue: string
  onModelChange: (value: string) => void
  isModelLoading?: boolean
  groups: GroupOption[]
  groupValue: string
  onGroupChange: (value: string) => void
  hasMessages?: boolean
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onClearMessages?: () => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  parameterEnabled: ParameterEnabled
}

export function PlaygroundInput({
  config,
  onSubmit,
  onStop,
  disabled,
  isGenerating,
  models,
  modelValue,
  onModelChange,
  isModelLoading = false,
  groups,
  groupValue,
  onGroupChange,
  hasMessages = false,
  onConfigChange,
  onClearMessages,
  onParameterEnabledChange,
  parameterEnabled,
}: PlaygroundInputProps) {
  const { t } = useTranslation()
  const [text, setText] = useState('')
  const { plugins } = useEnabledPlugins()
  const [mention, setMention] = useState<{
    start: number
    query: string
  } | null>(null)
  const [activeIndex, setActiveIndex] = useState(0)

  const mentionMatches = mention ? filterPluginMentions(plugins, mention.query) : []

  const scanMentionAtCaret = (textarea: HTMLTextAreaElement) => {
    const caret = textarea.selectionStart ?? textarea.value.length
    const before = textarea.value.slice(0, caret)
    const match = /(?:^|[\s])@([a-z0-9-]{0,64})$/.exec(before)
    setMention(
      match ? { start: caret - match[1].length - 1, query: match[1] } : null
    )
    setActiveIndex(0)
  }

  const handleTextChange = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
    setText(event.target.value)
    scanMentionAtCaret(event.target)
  }

  // Arrow keys and mouse clicks move the caret without firing onChange;
  // rescan so a later Enter never splices at a stale mention start.
  const handleCaretMove = (
    event: React.SyntheticEvent<HTMLTextAreaElement>
  ) => {
    if (!mention) return
    scanMentionAtCaret(event.currentTarget)
  }

  // On keyup only rescan for keys that actually move the caret; other keys
  // (typing, or ArrowUp/ArrowDown consumed by the dropdown) must not reset
  // activeIndex while the mention is unchanged.
  const handleKeyUp = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (!CARET_MOVE_KEYS.has(event.key)) return
    handleCaretMove(event)
  }

  const insertMention = (slug: string) => {
    if (!mention) return
    const next = `${text.slice(0, mention.start)}@${slug} ${text.slice(mention.start + mention.query.length + 1)}`
    setText(next)
    setMention(null)
  }

  // Capture phase so the dropdown can consume Arrow/Enter/Escape before the
  // textarea's own Enter-to-submit handler runs.
  const handleKeyDownCapture = (
    event: React.KeyboardEvent<HTMLDivElement>
  ) => {
    // Only intercept keys typed into the textarea; the mention dropdown must
    // not swallow keys from footer controls (model/group selects) that
    // bubble through this wrapper, and must not disturb IME composition.
    if (
      !(event.target instanceof HTMLTextAreaElement) ||
      event.nativeEvent.isComposing
    ) {
      return
    }

    if (event.key === 'Escape') {
      if (!mention) return
      event.preventDefault()
      event.stopPropagation()
      setMention(null)
      return
    }

    if (!mention || mentionMatches.length === 0) return

    if (event.key === 'ArrowDown') {
      event.preventDefault()
      event.stopPropagation()
      setActiveIndex((index) => (index + 1) % mentionMatches.length)
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      event.stopPropagation()
      setActiveIndex(
        (index) =>
          (index - 1 + mentionMatches.length) % mentionMatches.length
      )
      return
    }
    if (event.key === 'Enter' || event.key === 'Tab') {
      event.preventDefault()
      event.stopPropagation()
      insertMention(mentionMatches[Math.min(activeIndex, mentionMatches.length - 1)].slug)
    }
  }

  const handleSubmit = (message: PromptInputMessage) => {
    const submittableText = getSubmittableInputText(message, disabled)

    if (!submittableText) return
    onSubmit(submittableText)
    setText('')
    setMention(null)
  }

  return (
    <div
      className='grid shrink-0 gap-4 px-1 md:pb-4'
      onKeyDownCapture={handleKeyDownCapture}
    >
      <PromptInput
        className='relative'
        groupClassName='overflow-visible bg-background/95 dark:bg-background/80 border-border/70 shadow-[0_18px_60px_-32px_rgba(0,0,0,0.65)] ring-1 ring-foreground/5 rounded-xl transition-all duration-200 focus-within:border-primary/45 focus-within:ring-primary/15 focus-within:shadow-[0_22px_70px_-34px_rgba(0,0,0,0.75)]'
        onSubmit={handleSubmit}
      >
        <PromptInputTextarea
          autoComplete='off'
          autoCorrect='off'
          autoCapitalize='off'
          spellCheck={false}
          className='min-h-20 px-5 pt-4 pb-3 leading-7 md:min-h-24 md:text-base'
          disabled={disabled}
          onChange={handleTextChange}
          onClick={handleCaretMove}
          onKeyUp={handleKeyUp}
          onSelect={handleCaretMove}
          placeholder={t('Ask anything')}
          value={text}
        />

        {mention && mentionMatches.length > 0 && (
          <PluginMentionDropdown
            activeIndex={Math.min(activeIndex, mentionMatches.length - 1)}
            matches={mentionMatches}
            onActiveIndexChange={setActiveIndex}
            onSelect={insertMention}
          />
        )}

        <PromptInputFooter className='border-border/60 bg-muted/20 dark:bg-muted/10 border-t px-3 py-2.5 backdrop-blur'>
          <PlaygroundInputControls
            disabled={disabled}
            groups={groups}
            groupValue={groupValue}
            isGenerating={isGenerating}
            isModelLoading={isModelLoading}
            models={models}
            modelValue={modelValue}
            onGroupChange={onGroupChange}
            onModelChange={onModelChange}
            onStop={onStop}
            text={text}
            tools={
              <PlaygroundInputTools
                config={config}
                disabled={disabled}
                hasMessages={hasMessages}
                onConfigChange={onConfigChange}
                onClearMessages={onClearMessages}
                onParameterEnabledChange={onParameterEnabledChange}
                parameterEnabled={parameterEnabled}
              />
            }
          />
        </PromptInputFooter>
      </PromptInput>
    </div>
  )
}
