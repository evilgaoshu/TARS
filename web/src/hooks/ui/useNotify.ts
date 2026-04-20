import { useMemo } from "react"
import { useToast } from "./use-toast"
import { getApiErrorMessage } from "../../lib/api/ops"

/**
 * Enterprise-grade hook for standardized notifications.
 * Connects API errors and success messages to the global Toast system.
 */
export function useNotify() {
  const { toast } = useToast()

  return useMemo(() => ({
    success: (message: string, title = "Operation Successful") => {
      toast({
        title,
        description: message,
        variant: "success",
      })
    },

    error: (err: unknown, fallback = "An unexpected error occurred") => {
      const message = getApiErrorMessage(err, fallback)
      toast({
        title: "Operation Failed",
        description: message,
        variant: "destructive",
      })
    },

    warn: (message: string, title = "Attention Required") => {
      toast({
        title,
        description: message,
        variant: "glass",
      })
    },
  }), [toast])
}
