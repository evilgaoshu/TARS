/* eslint-disable @typescript-eslint/no-explicit-any */
import { useForm } from 'react-hook-form';
import type { DefaultValues, Path } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from './form';
import { Input } from './input';
import { Checkbox } from './checkbox';
import { NativeSelect } from './select';
import { Textarea } from './textarea';
import { Button } from './button';
import { Save, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';

interface AutoFormProps<T extends z.ZodObject<any>> {
  schema: T;
  onSubmit: (values: z.infer<T>) => void | Promise<void>;
  defaultValues?: DefaultValues<z.infer<T>>;
  fieldConfig?: Partial<Record<keyof z.infer<T>, {
    label?: string;
    description?: string;
    placeholder?: string;
    type?: 'text' | 'password' | 'email' | 'number' | 'textarea' | 'select' | 'checkbox';
    options?: { label: string; value: unknown }[];
    hidden?: boolean;
    disabled?: boolean;
    className?: string;
  }>>;
  submitLabel?: string;
  submitting?: boolean;
  className?: string;
}

/**
 * Enterprise-grade AutoForm
 * Generates high-quality forms directly from Zod schemas with zero manual JSX.
 */
export function AutoForm<T extends z.ZodObject<any>>({
  schema,
  onSubmit,
  defaultValues,
  fieldConfig = {},
  submitLabel = 'Save Changes',
  submitting = false,
  className,
}: AutoFormProps<T>) {
  const form = useForm<z.infer<T>>({
    resolver: zodResolver(schema) as any,
    defaultValues,
  });

  const renderField = (name: string, shape: z.ZodTypeAny) => {
    const config = (fieldConfig as any)[name] || {};
    if (config.hidden) return null;

    return (
      <FormField
        key={name}
        control={form.control}
        name={name as Path<z.infer<T>>}
        render={({ field }) => {
          // 1. Determine Control Type
          let control = null;
          const label = config.label || name.split('_').map(s => s.charAt(0).toUpperCase() + s.slice(1)).join(' ');

          if (shape instanceof z.ZodBoolean || config.type === 'checkbox') {
            control = (
              <FormItem className={cn("flex flex-row items-start space-x-3 space-y-0 rounded-xl border border-white/5 bg-white/5 p-4 transition-all hover:bg-white/[0.08]", config.className)}>
                <FormControl>
                  <Checkbox checked={!!field.value} onCheckedChange={field.onChange} disabled={config.disabled} />
                </FormControl>
                <div className="space-y-1 leading-none">
                  <FormLabel>{label}</FormLabel>
                  {config.description && <FormDescription>{config.description}</FormDescription>}
                </div>
              </FormItem>
            );
          } else if (shape instanceof z.ZodEnum || config.type === 'select') {
            const options = config.options || (shape instanceof z.ZodEnum ? (shape as any).options.map((v: string) => ({ label: v, value: v })) : []);
            control = (
              <FormItem className={config.className}>
                <FormLabel>{label}</FormLabel>
                <FormControl>
                  <NativeSelect 
                    className="bg-bg-surface-solid w-full" 
                    {...field}
                    value={(field.value as string) || ''}
                    disabled={config.disabled}
                  >
                    {options.map((opt: { label: string; value: unknown }) => (
                      <option key={String(opt.value)} value={String(opt.value)}>{opt.label}</option>
                    ))}
                  </NativeSelect>
                </FormControl>
                {config.description && <FormDescription>{config.description}</FormDescription>}
                <FormMessage />
              </FormItem>
            );
          } else if (config.type === 'textarea') {
            control = (
              <FormItem className={config.className}>
                <FormLabel>{label}</FormLabel>
                <FormControl>
                  <Textarea 
                    className="min-h-[100px] w-full py-2" 
                    placeholder={config.placeholder} 
                    disabled={config.disabled}
                    {...field}
                    value={(field.value as string) || ''}
                  />
                </FormControl>
                {config.description && <FormDescription>{config.description}</FormDescription>}
                <FormMessage />
              </FormItem>
            );
          } else {
            // Default: Input
            const type = config.type || (name.toLowerCase().includes('password') || name.toLowerCase().includes('key') ? 'password' : 'text');
            control = (
              <FormItem className={config.className}>
                <FormLabel>{label}</FormLabel>
                <FormControl>
                  <Input 
                    type={type} 
                    placeholder={config.placeholder} 
                    disabled={config.disabled}
                    autoComplete={type === 'password' ? 'new-password' : 'off'}
                    {...field}
                    value={(field.value as string) || ''}
                  />
                </FormControl>
                {config.description && <FormDescription>{config.description}</FormDescription>}
                <FormMessage />
              </FormItem>
            );
          }

          return control;
        }}
      />
    );
  };

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className={cn("space-y-6", className)}>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {Object.keys(schema.shape).map(key => renderField(key, schema.shape[key]))}
        </div>
        
        <div className="flex gap-3 pt-4 border-t border-white/5">
          <Button type="submit" variant="primary" className="px-8" disabled={submitting}>
            {submitting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Save className="mr-2 h-4 w-4" />}
            {submitLabel}
          </Button>
          <Button type="button" variant="ghost" onClick={() => form.reset()}>
            Reset
          </Button>
        </div>
      </form>
    </Form>
  );
}
