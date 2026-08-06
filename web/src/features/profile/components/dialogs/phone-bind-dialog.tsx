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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { handleServerError } from '@/lib/handle-server-error'

import { updateUserProfile } from '../../api'
import { phoneBindFormSchema, type PhoneBindFormData } from '../../lib'

// ============================================================================
// Phone Bind Dialog Component
// ============================================================================

interface PhoneBindDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentPhone?: string
  onSuccess: () => void
}

export function PhoneBindDialog({
  open,
  onOpenChange,
  currentPhone,
  onSuccess,
}: PhoneBindDialogProps) {
  const { t } = useTranslation()

  const form = useForm<PhoneBindFormData>({
    resolver: zodResolver(phoneBindFormSchema),
    defaultValues: { phone: '' },
  })

  const saveMutation = useMutation({
    mutationFn: (values: PhoneBindFormData) =>
      updateUserProfile({ phone: values.phone }),
    onSuccess: (response) => {
      if (response.success) {
        toast.success(t('Phone updated'))
        onOpenChange(false)
        onSuccess()
        form.reset()
      }
      // Business errors (success: false) are already toasted with a
      // localized message by the http-client response interceptor.
    },
    onError: handleServerError,
  })

  // Empty submission is rejected by the schema: self-service unbind is not
  // supported (UpdateWithTx skips zero values, so it would be a backend no-op).
  const onSubmit = (values: PhoneBindFormData) => {
    saveMutation.mutate(values)
  }

  const handleOpenChange = (open: boolean) => {
    if (!saveMutation.isPending) {
      onOpenChange(open)
      if (!open) {
        // Reset form when closing
        form.reset()
      }
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t('Bind Phone')}
      description={
        currentPhone
          ? t('Current phone: {{phone}}. Enter a new phone number to change.', {
              phone: currentPhone,
            })
          : t('Bind a phone number to your account.')
      }
      contentClassName='sm:max-w-md'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => handleOpenChange(false)}
            disabled={saveMutation.isPending}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='phone-bind-form'
            disabled={saveMutation.isPending}
          >
            {saveMutation.isPending && (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            )}
            {saveMutation.isPending ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id='phone-bind-form'
          onSubmit={form.handleSubmit(onSubmit)}
          className='space-y-4 py-4'
        >
          <FormField
            control={form.control}
            name='phone'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Phone')}</FormLabel>
                <FormControl>
                  <Input
                    type='tel'
                    placeholder={t('Enter your phone number')}
                    disabled={saveMutation.isPending}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
                {currentPhone && (
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'To remove a bound phone number, please contact an administrator.'
                    )}
                  </p>
                )}
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}
