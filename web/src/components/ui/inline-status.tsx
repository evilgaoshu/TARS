import { Alert, AlertDescription } from "@/components/ui/alert"

const variantMap = {
  info: "info",
  success: "success",
  warning: "warning",
  error: "destructive",
} as const

export function InlineStatus({
  message,
  type = "info",
}: {
  message: string
  type?: "info" | "success" | "warning" | "error"
}) {
  return (
    <Alert variant={variantMap[type]}>
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  )
}
