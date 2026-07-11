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
import { useCallback, useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Server, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { getUserModels, getUserGroups } from './api'
import { PlaygroundChat } from './components/playground-chat'
import { PlaygroundInput } from './components/playground-input'
import { PLAYGROUND_ENDPOINTS } from './constants'
import { usePlaygroundState, useChatHandler } from './hooks'
import { createUserMessage, createLoadingAssistantMessage } from './lib'
import type { Message as MessageType, PlaygroundEndpoint } from './types'

const CLEAR_CHAT_CONFIRMATION = 'CLEAR'

export function Playground() {
  const { t } = useTranslation()
  const {
    config,
    parameterEnabled,
    messages,
    models,
    groups,
    updateMessages,
    setModels,
    setGroups,
    updateConfig,
    clearMessages,
  } = usePlaygroundState()

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

  // Edit dialog state
  const [editingMessageKey, setEditingMessageKey] = useState<string | null>(
    null
  )
  const [isClearDialogOpen, setIsClearDialogOpen] = useState(false)
  const [clearConfirmationText, setClearConfirmationText] = useState('')

  // Load models
  const { data: modelsData, isLoading: isLoadingModels } = useQuery({
    queryKey: ['playground-models', t],
    queryFn: async () => {
      try {
        return await getUserModels()
      } catch (error) {
        toast.error(
          error instanceof Error
            ? error.message
            : t('Failed to load playground models')
        )
        return []
      }
    },
  })

  // Load groups
  const { data: groupsData } = useQuery({
    queryKey: ['playground-groups', t],
    queryFn: async () => {
      try {
        return await getUserGroups()
      } catch (error) {
        toast.error(
          error instanceof Error
            ? error.message
            : t('Failed to load playground groups')
        )
        return []
      }
    },
  })

  // Update models when data changes
  useEffect(() => {
    if (!modelsData) return

    setModels(modelsData)

    // Set default model if current model is not available
    const isCurrentModelValid = modelsData.some((m) => m.value === config.model)
    if (modelsData.length > 0 && !isCurrentModelValid) {
      updateConfig('model', modelsData[0].value)
    }
  }, [modelsData, config.model, setModels, updateConfig])

  // Update groups when data changes
  useEffect(() => {
    if (!groupsData) return

    setGroups(groupsData)

    const hasCurrentGroup = groupsData.some((g) => g.value === config.group)
    if (!hasCurrentGroup && groupsData.length > 0) {
      const fallback =
        groupsData.find((g) => g.value === 'default')?.value ??
        groupsData[0].value
      updateConfig('group', fallback)
    }
  }, [groupsData, setGroups, config.group, updateConfig])

  const handleSendMessage = (text: string) => {
    const userMessage = createUserMessage(text)
    const assistantMessage = createLoadingAssistantMessage()

    const newMessages = [...messages, userMessage, assistantMessage]
    updateMessages(newMessages)

    // Send chat request
    sendChat(newMessages)
  }

  const handleCopyMessage = (message: MessageType) => {
    // Copy is handled in MessageActions component
    // eslint-disable-next-line no-console
    console.log('Message copied:', message.key)
  }

  const handleRegenerateMessage = (message: MessageType) => {
    // Find the message index and regenerate from there
    const messageIndex = messages.findIndex((m) => m.key === message.key)
    if (messageIndex === -1) return

    // Remove messages after this one and regenerate
    const messagesUpToHere = messages.slice(0, messageIndex)
    const loadingMessage = createLoadingAssistantMessage()
    const newMessages = [...messagesUpToHere, loadingMessage]

    updateMessages(newMessages)
    sendChat(newMessages)
  }

  const handleEditMessage = useCallback((message: MessageType) => {
    setEditingMessageKey(message.key)
  }, [])

  const handleEditOpenChange = useCallback((open: boolean) => {
    if (!open) setEditingMessageKey(null)
  }, [])

  // Apply edit and optionally re-submit from the edited user message
  const applyEdit = useCallback(
    (newContent: string, submit: boolean) => {
      if (!editingMessageKey) return
      const index = messages.findIndex((m) => m.key === editingMessageKey)
      if (index === -1) return

      const updated = messages.map((m) =>
        m.key === editingMessageKey
          ? { ...m, versions: [{ ...m.versions[0], content: newContent }] }
          : m
      )

      setEditingMessageKey(null)

      if (!submit || updated[index].from !== 'user') {
        updateMessages(updated)
        return
      }

      const toSubmit = [
        ...updated.slice(0, index + 1),
        createLoadingAssistantMessage(),
      ]
      updateMessages(toSubmit)
      sendChat(toSubmit)
    },
    [editingMessageKey, messages, updateMessages, sendChat]
  )

  const handleDeleteMessage = (message: MessageType) => {
    const newMessages = messages.filter((m) => m.key !== message.key)
    updateMessages(newMessages)
  }

  const handleClearDialogOpenChange = useCallback((open: boolean) => {
    setIsClearDialogOpen(open)
    if (!open) {
      setClearConfirmationText('')
    }
  }, [])

  const handleClearMessages = useCallback(() => {
    clearMessages()
    setEditingMessageKey(null)
    handleClearDialogOpenChange(false)
    toast.success(t('Chat history cleared'))
  }, [clearMessages, handleClearDialogOpenChange, t])

  const handleEndpointChange = useCallback(
    (value: string | null) => {
      if (!value) return
      updateConfig('endpoint', value as PlaygroundEndpoint)
    },
    [updateConfig]
  )

  return (
    <div className='relative flex size-full flex-col overflow-hidden'>
      <div className='bg-background/95 border-b px-4 py-2'>
        <div className='mx-auto flex w-full max-w-4xl flex-wrap items-center justify-between gap-2 sm:gap-3'>
          <div className='min-w-0'>
            <div className='text-sm leading-tight font-medium'>
              {t('Playground')}
            </div>
          </div>
          <div className='flex min-w-0 shrink-0 flex-wrap items-center justify-end gap-2'>
            <Tabs
              value={config.endpoint}
              onValueChange={handleEndpointChange}
              className='shrink-0'
            >
              <TabsList className='h-8'>
                <TabsTrigger
                  value={PLAYGROUND_ENDPOINTS.CHAT_COMPLETIONS}
                  disabled={isGenerating}
                  className='px-2.5 text-xs'
                >
                  {t('Chat')}
                </TabsTrigger>
                <TabsTrigger
                  value={PLAYGROUND_ENDPOINTS.RESPONSES}
                  disabled={isGenerating}
                  className='px-2.5 text-xs'
                >
                  {t('Responses')}
                </TabsTrigger>
              </TabsList>
            </Tabs>
            <Button
              size='sm'
              variant='outline'
              disabled={isGenerating || messages.length === 0}
              onClick={() => handleClearDialogOpenChange(true)}
            >
              <Trash2 className='size-4' aria-hidden='true' />
              <span className='hidden sm:inline'>{t('Clear')}</span>
              <span className='sr-only sm:hidden'>{t('Clear')}</span>
            </Button>
            <Button
              size='sm'
              variant='outline'
              render={<Link to='/playground/provider-check' />}
            >
              <Server className='size-4' aria-hidden='true' />
              <span className='hidden sm:inline'>{t('Provider Check')}</span>
              <span className='sr-only sm:hidden'>{t('Provider Check')}</span>
            </Button>
          </div>
        </div>
      </div>

      <ConfirmDialog
        open={isClearDialogOpen}
        onOpenChange={handleClearDialogOpenChange}
        title={t('Clear all chat history?')}
        desc={
          <span>
            {t(
              'This will remove all Playground chat messages from this browser. Type {{value}} to confirm.',
              {
                value: CLEAR_CHAT_CONFIRMATION,
              }
            )}
          </span>
        }
        destructive
        confirmText={t('Clear history')}
        disabled={clearConfirmationText.trim() !== CLEAR_CHAT_CONFIRMATION}
        handleConfirm={handleClearMessages}
      >
        <Input
          autoComplete='off'
          value={clearConfirmationText}
          onChange={(event) => setClearConfirmationText(event.target.value)}
          placeholder={CLEAR_CHAT_CONFIRMATION}
          aria-label={t('Confirmation text')}
        />
      </ConfirmDialog>

      {/* Full-width scroll container: scrolling works even over side whitespace */}
      <div className='flex flex-1 flex-col overflow-hidden'>
        <PlaygroundChat
          messages={messages}
          onCopyMessage={handleCopyMessage}
          onRegenerateMessage={handleRegenerateMessage}
          onEditMessage={handleEditMessage}
          onDeleteMessage={handleDeleteMessage}
          isGenerating={isGenerating}
          editingKey={editingMessageKey}
          onCancelEdit={handleEditOpenChange}
          onSaveEdit={(newContent) => applyEdit(newContent, false)}
          onSaveEditAndSubmit={(newContent) => applyEdit(newContent, true)}
        />
      </div>

      {/* Input area: center content and constrain to the same container width */}
      <div className='mx-auto w-full max-w-4xl'>
        <PlaygroundInput
          disabled={isGenerating}
          groups={groups}
          groupValue={config.group}
          isGenerating={isGenerating}
          isModelLoading={isLoadingModels}
          modelValue={config.model}
          models={models}
          onGroupChange={(value) => updateConfig('group', value)}
          onModelChange={(value) => updateConfig('model', value)}
          onStop={stopGeneration}
          onSubmit={handleSendMessage}
        />
      </div>
    </div>
  )
}
