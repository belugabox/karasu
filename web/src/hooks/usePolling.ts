import { useEffect } from 'react'

export function usePolling(
  task: () => Promise<void> | void,
  intervalMs: number,
  enabled = true,
): void {
  useEffect(() => {
    if (!enabled) {
      return
    }

    let disposed = false

    const run = async () => {
      if (disposed) {
        return
      }
      await task()
    }

    void run()
    const timerId = window.setInterval(() => {
      void run()
    }, intervalMs)

    return () => {
      disposed = true
      window.clearInterval(timerId)
    }
  }, [task, intervalMs, enabled])
}
