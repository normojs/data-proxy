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
import { useEffect, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  ArrowDown,
  ArrowUp,
  Edit,
  ExternalLink,
  Plus,
  Save,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { StatusBadge } from '@/components/status-badge'
import {
  DownloadIcon,
  DOWNLOAD_ICON_OPTIONS,
  parseDownloadItems,
} from '@/features/downloads/lib'
import {
  DOWNLOAD_ICON_KEYS,
  type DownloadIconKey,
  type DownloadItem,
} from '@/features/downloads/types'
import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type DownloadsSectionProps = {
  enabled: boolean
  data: string
}

const isHttpUrl = (value: string) => {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

const createDownloadSchema = (t: (key: string) => string) =>
  z.object({
    name: z
      .string()
      .trim()
      .min(1, t('Software name is required'))
      .max(80, t('Software name must be 80 characters or less')),
    description: z
      .string()
      .trim()
      .max(240, t('Software description must be 240 characters or less')),
    url: z
      .string()
      .trim()
      .min(1, t('Download URL is required'))
      .refine(isHttpUrl, t('Must be a valid HTTP or HTTPS URL')),
    icon: z.enum(DOWNLOAD_ICON_KEYS),
    customIconUrl: z
      .string()
      .trim()
      .max(500, t('Custom icon URL must be 500 characters or less'))
      .refine(
        (value) => value === '' || isHttpUrl(value),
        t('Must be a valid HTTP or HTTPS URL')
      ),
    openInNewTab: z.boolean(),
    enabled: z.boolean(),
  })

type DownloadFormValues = z.infer<ReturnType<typeof createDownloadSchema>>

const emptyDownloadFormValues: DownloadFormValues = {
  name: '',
  description: '',
  url: '',
  icon: 'download',
  customIconUrl: '',
  openInNewTab: true,
  enabled: true,
}

function toFormValues(item: DownloadItem): DownloadFormValues {
  return {
    name: item.name,
    description: item.description ?? '',
    url: item.url,
    icon: item.icon,
    customIconUrl: item.customIconUrl ?? '',
    openInNewTab: item.openInNewTab,
    enabled: item.enabled,
  }
}

function getNextId(items: DownloadItem[]) {
  return Math.max(...items.map((item) => item.id), 0) + 1
}

export function DownloadsSection({ enabled, data }: DownloadsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const downloadSchema = createDownloadSchema(t)
  const [downloadList, setDownloadList] = useState<DownloadItem[]>([])
  const [isEnabled, setIsEnabled] = useState(enabled)
  const [hasChanges, setHasChanges] = useState(false)
  const [showDialog, setShowDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [editingDownload, setEditingDownload] = useState<DownloadItem | null>(
    null
  )

  const form = useForm<DownloadFormValues>({
    resolver: zodResolver(downloadSchema),
    defaultValues: emptyDownloadFormValues,
  })

  useEffect(() => {
    setDownloadList(parseDownloadItems(data))
    setHasChanges(false)
  }, [data])

  useEffect(() => {
    setIsEnabled(enabled)
  }, [enabled])

  const handleToggleEnabled = async (checked: boolean) => {
    try {
      const result = await updateOption.mutateAsync({
        key: 'console_setting.downloads_enabled',
        value: checked,
      })
      if (result.success) {
        setIsEnabled(checked)
      }
    } catch {
      toast.error(t('Failed to update setting'))
    }
  }

  const handleAdd = () => {
    setEditingDownload(null)
    form.reset(emptyDownloadFormValues)
    setShowDialog(true)
  }

  const handleEdit = (item: DownloadItem) => {
    setEditingDownload(item)
    form.reset(toFormValues(item))
    setShowDialog(true)
  }

  const handleDelete = (item: DownloadItem) => {
    setEditingDownload(item)
    setShowDeleteDialog(true)
  }

  const handleMove = (id: number, direction: -1 | 1) => {
    setDownloadList((prev) => {
      const index = prev.findIndex((item) => item.id === id)
      const targetIndex = index + direction
      if (index < 0 || targetIndex < 0 || targetIndex >= prev.length) {
        return prev
      }
      const next = [...prev]
      const moved = next[index]
      next[index] = next[targetIndex]
      next[targetIndex] = moved
      return next
    })
    setHasChanges(true)
  }

  const handleSubmitForm = (values: DownloadFormValues) => {
    const itemValues = {
      ...values,
      icon: values.icon as DownloadIconKey,
    }

    if (editingDownload) {
      setDownloadList((prev) =>
        prev.map((item) =>
          item.id === editingDownload.id ? { ...item, ...itemValues } : item
        )
      )
      toast.success(t('Download updated. Click "Save Settings" to apply.'))
    } else {
      setDownloadList((prev) => [
        ...prev,
        {
          id: getNextId(prev),
          ...itemValues,
        },
      ])
      toast.success(t('Download added. Click "Save Settings" to apply.'))
    }

    setHasChanges(true)
    setShowDialog(false)
    setEditingDownload(null)
  }

  const confirmDelete = () => {
    if (!editingDownload) return

    setDownloadList((prev) =>
      prev.filter((item) => item.id !== editingDownload.id)
    )
    setHasChanges(true)
    setShowDeleteDialog(false)
    setEditingDownload(null)
    toast.success(t('Download deleted. Click "Save Settings" to apply.'))
  }

  const handleSaveAll = async () => {
    try {
      const result = await updateOption.mutateAsync({
        key: 'console_setting.downloads',
        value: JSON.stringify(downloadList),
      })
      if (result.success) {
        setHasChanges(false)
      }
    } catch {
      toast.error(t('Failed to save downloads'))
    }
  }

  return (
    <SettingsSection title={t('Downloads')}>
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <Button onClick={handleAdd} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('Add download')}
            </Button>
            <Button
              onClick={handleSaveAll}
              size='sm'
              variant='secondary'
              disabled={!hasChanges || updateOption.isPending}
            >
              <Save className='mr-2 h-4 w-4' />
              {updateOption.isPending ? t('Saving...') : t('Save Settings')}
            </Button>
          </div>
          <SettingsSwitchField
            checked={isEnabled}
            onCheckedChange={handleToggleEnabled}
            label={t('Enabled')}
            description={t('Show the public downloads page and its links.')}
            className='border-b-0 py-0'
          />
        </div>

        <div className='rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='w-24'>{t('Status')}</TableHead>
                <TableHead className='w-20'>{t('Icon')}</TableHead>
                <TableHead>{t('Software')}</TableHead>
                <TableHead>{t('Download URL')}</TableHead>
                <TableHead className='w-28'>{t('New tab')}</TableHead>
                <TableHead className='w-44'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {downloadList.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className='h-24 text-center'>
                    {t(
                      'No downloads yet. Click "Add download" to publish one.'
                    )}
                  </TableCell>
                </TableRow>
              ) : (
                downloadList.map((item, index) => (
                  <TableRow key={item.id}>
                    <TableCell>
                      <StatusBadge
                        label={item.enabled ? t('Enabled') : t('Disabled')}
                        variant={item.enabled ? 'success' : 'neutral'}
                        copyable={false}
                      />
                    </TableCell>
                    <TableCell>
                      <div className='bg-muted text-muted-foreground flex size-8 items-center justify-center rounded-lg'>
                        <DownloadIcon item={item} />
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='min-w-0'>
                        <div className='max-w-52 truncate text-sm font-medium'>
                          {item.name}
                        </div>
                        {item.description ? (
                          <div className='text-muted-foreground max-w-72 truncate text-xs'>
                            {item.description}
                          </div>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      <a
                        href={item.url}
                        target='_blank'
                        rel='noopener noreferrer'
                        className='text-muted-foreground hover:text-foreground inline-flex max-w-80 items-center gap-1 truncate font-mono text-xs'
                        title={item.url}
                      >
                        <span className='truncate'>{item.url}</span>
                        <ExternalLink className='size-3 shrink-0' />
                      </a>
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        label={item.openInNewTab ? t('Yes') : t('No')}
                        variant={item.openInNewTab ? 'info' : 'neutral'}
                        copyable={false}
                      />
                    </TableCell>
                    <TableCell>
                      <div className='flex items-center gap-1'>
                        <Button
                          onClick={() => handleMove(item.id, -1)}
                          size='icon-sm'
                          variant='ghost'
                          disabled={index === 0}
                          title={t('Move up')}
                        >
                          <ArrowUp className='h-4 w-4' />
                        </Button>
                        <Button
                          onClick={() => handleMove(item.id, 1)}
                          size='icon-sm'
                          variant='ghost'
                          disabled={index === downloadList.length - 1}
                          title={t('Move down')}
                        >
                          <ArrowDown className='h-4 w-4' />
                        </Button>
                        <Button
                          onClick={() => handleEdit(item)}
                          size='icon-sm'
                          variant='ghost'
                          title={t('Edit download')}
                        >
                          <Edit className='h-4 w-4' />
                        </Button>
                        <Button
                          onClick={() => handleDelete(item)}
                          size='icon-sm'
                          variant='ghost'
                          title={t('Delete download')}
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>

      <Dialog open={showDialog} onOpenChange={setShowDialog}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>
              {editingDownload ? t('Edit download') : t('Add download')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'Configure companion software links shown on the public downloads page.'
              )}
            </DialogDescription>
          </DialogHeader>
          <Form {...form}>
            <form
              onSubmit={form.handleSubmit(handleSubmitForm)}
              className='grid gap-4 md:grid-cols-2'
            >
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Software name')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('e.g., Codex')} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='icon'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Icon')}</FormLabel>
                    <Select
                      items={DOWNLOAD_ICON_OPTIONS}
                      onValueChange={field.onChange}
                      value={field.value}
                    >
                      <FormControl>
                        <SelectTrigger className='w-full'>
                          <SelectValue placeholder={t('Select an icon')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {DOWNLOAD_ICON_OPTIONS.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              <span className='inline-flex items-center gap-2'>
                                <DownloadIcon
                                  item={{
                                    name: option.label,
                                    icon: option.value,
                                  }}
                                  className='size-4'
                                />
                                {t(option.label)}
                              </span>
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='url'
                render={({ field }) => (
                  <FormItem className='md:col-span-2'>
                    <FormLabel>{t('Download URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('https://example.com/download')}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem className='md:col-span-2'>
                    <FormLabel>{t('Software description')}</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'Short note shown below the software name'
                        )}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Optional. Keep it short so the public list stays easy to scan.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='customIconUrl'
                render={({ field }) => (
                  <FormItem className='md:col-span-2'>
                    <FormLabel>{t('Custom icon URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('https://example.com/icon.png')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Optional. Overrides the selected built-in icon.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='openInNewTab'
                render={({ field }) => (
                  <FormItem className='flex min-w-0 items-center justify-between gap-4 rounded-lg border p-3'>
                    <div className='min-w-0 space-y-0.5'>
                      <FormLabel>{t('Open in new tab')}</FormLabel>
                      <FormDescription>
                        {t('Keep users on Data Proxy after opening downloads.')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem className='flex min-w-0 items-center justify-between gap-4 rounded-lg border p-3'>
                    <div className='min-w-0 space-y-0.5'>
                      <FormLabel>{t('Enabled')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Disabled items stay saved but are hidden publicly.'
                        )}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <DialogFooter className='md:col-span-2'>
                <Button
                  type='button'
                  variant='outline'
                  onClick={() => setShowDialog(false)}
                >
                  {t('Cancel')}
                </Button>
                <Button type='submit'>
                  {editingDownload ? t('Update') : t('Add')}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete download?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('This download link will be removed from the list.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
