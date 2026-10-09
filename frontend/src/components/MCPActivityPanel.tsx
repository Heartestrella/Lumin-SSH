import { useCallback, useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { Activity } from 'lucide-react'
import { useTranslation, type I18nKey } from '../i18n.js'
import { Z } from '../constants/zIndex.ts'
import { Button } from './ui'

export interface MCPActivityEvent {
  requestId: string
  source: string
  clientName: string
  tool: string
  sessionId: string
  serverName: string
  command?: string
  purpose?: string
  isMutating: boolean
  cwd?: string
  status: string
  output?: string
  exitCode?: number | null
  timestamp: number
}

interface ActivityCard {
  events: MCPActivityEvent[]
  resolved: boolean
}

type ActivityMap = Map<string, ActivityCard>

const statusColors: Record<string, string> = {
  started: 'var(--accent)',
  queued: 'var(--warning)',
  running: 'var(--accent)',
  output: 'var(--text-muted)',
  done: 'var(--success)',
  error: 'var(--danger)',
  approval_required: 'var(--warning)',
  approved: 'var(--success)',
  rejected: 'var(--danger)',
  timed_out: 'var(--danger)',
}

const statusLabels: Record<string, string> = {
  started: '开始',
  queued: '排队中',
  running: '执行中',
  output: '等待处理',
  done: '已完成',
  error: '错误',
  approval_required: '等待审批',
  approved: '已批准',
  rejected: '已拒绝',
  timed_out: '审批超时',
}

const clientColors: Record<string, string> = {
  'claude-code': '#d97757',
  'codex': '#10a37f',
  'cursor': '#5b9cf6',
  'cline': '#a78bfa',
  'windsurf': '#5b9cf6',
}

function formatTime(ts: number): string {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

async function resolveApproval(requestId: string, approved: boolean): Promise<void> {
  const w = window as unknown as {
    go?: { wailsapp?: { App?: { ResolveMCPApproval?: (id: string, a: boolean) => Promise<void> } } }
  }
  const resolver = w.go?.wailsapp?.App?.ResolveMCPApproval
  if (typeof resolver !== 'function') {
    throw new Error('MCP approval bridge is unavailable')
  }
  await resolver.call(w.go?.wailsapp?.App, requestId, approved)
}

export interface MCPActivityPanelProps {
  height?: string
  onClose?: () => void
  /** 审批请求到来时回调（弹窗被关闭时用于自动弹出） */
  onApprovalRequired?: () => void
  /** 标题栏按下（用于拖动整个弹窗） */
  onHeaderPointerDown?: (e: ReactPointerEvent<HTMLDivElement>) => void
  /** 标题栏双击（用于复位弹窗位置） */
  onHeaderDoubleClick?: () => void
}

export interface MCPActivityFloatingToggleProps {
  visible: boolean
  offset: { x: number; y: number }
  onClick: () => void
  onPointerDown: (e: { button?: number; clientX: number; clientY: number }) => void
  onDoubleClick: () => void
}

export function MCPActivityFloatingToggle({ visible, offset, onClick, onPointerDown, onDoubleClick }: MCPActivityFloatingToggleProps) {
  const { t } = useTranslation()
  if (!visible) return null
  return (
    <button
      onClick={onClick}
      onPointerDown={onPointerDown}
      onDoubleClick={onDoubleClick}
      title={t('拖动按钮移动，双击复位')}
      className="fixed bottom-4 right-4 w-10 h-10 rounded-full border border-line bg-overlay text-secondary cursor-grab flex items-center justify-center shadow-md select-none [touch-action:none]"
      style={{ zIndex: Z.SETTINGS, transform: `translate(${offset.x}px, ${offset.y}px)` }}
    >
      <Activity size={17} strokeWidth={2} />
    </button>
  )
}

export default function MCPActivityPanel({ height = '100%', onClose, onApprovalRequired, onHeaderPointerDown, onHeaderDoubleClick }: MCPActivityPanelProps) {
  const { t } = useTranslation()
  const [activities, setActivities] = useState<ActivityMap>(new Map())
  const activitiesRef = useRef<ActivityMap>(new Map())
  const [autoScroll, setAutoScroll] = useState(true)
  const [resolvingApprovals, setResolvingApprovals] = useState<Set<string>>(() => new Set())
  const [approvalErrors, setApprovalErrors] = useState<Map<string, string>>(() => new Map())
  const scrollRef = useRef<HTMLDivElement>(null)
  const cardOrderRef = useRef<string[]>([])
  const onApprovalRequiredRef = useRef(onApprovalRequired)
  useEffect(() => { onApprovalRequiredRef.current = onApprovalRequired }, [onApprovalRequired])

  const flushState = useCallback(() => {
    setActivities(new Map(activitiesRef.current))
  }, [])

  useEffect(() => {
    const unbind = EventsOn('mcp-activity', (payload: MCPActivityEvent) => {
      if (!payload?.requestId) return
      const map = activitiesRef.current
      const existing = map.get(payload.requestId)
      if (existing) {
        existing.events.push(payload)
        if (payload.status === 'approved' || payload.status === 'rejected' || payload.status === 'timed_out') {
          existing.resolved = true
          setApprovalErrors((current) => {
            if (!current.has(payload.requestId)) return current
            const next = new Map(current)
            next.delete(payload.requestId)
            return next
          })
        }
      } else {
        const resolved = payload.status === 'approved' || payload.status === 'rejected' || payload.status === 'timed_out'
        map.set(payload.requestId, { events: [payload], resolved })
        cardOrderRef.current.unshift(payload.requestId)
        if (cardOrderRef.current.length > 50) {
          const removed = cardOrderRef.current.pop()
          if (removed) {
            map.delete(removed)
            setResolvingApprovals((current) => {
              if (!current.has(removed)) return current
              const next = new Set(current)
              next.delete(removed)
              return next
            })
            setApprovalErrors((current) => {
              if (!current.has(removed)) return current
              const next = new Map(current)
              next.delete(removed)
              return next
            })
          }
        }
      }
      if (payload.status === 'approval_required') {
        onApprovalRequiredRef.current?.()
      }
      flushState()
    })
    return () => { unbind() }
  }, [flushState])

  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = 0
    }
  }, [activities, autoScroll])

  const handleScroll = useCallback(() => {
    if (!scrollRef.current) return
    setAutoScroll(scrollRef.current.scrollTop < 50)
  }, [])

  const handleResolveApproval = useCallback(async (requestId: string, approved: boolean, card: ActivityCard) => {
    setResolvingApprovals((current) => new Set(current).add(requestId))
    setApprovalErrors((current) => {
      const next = new Map(current)
      next.delete(requestId)
      return next
    })
    try {
      await resolveApproval(requestId, approved)
      card.resolved = true
      flushState()
    } catch (error) {
      if (activitiesRef.current.has(requestId)) {
        setApprovalErrors((current) => new Map(current).set(
          requestId,
          error instanceof Error ? error.message : String(error),
        ))
      }
    } finally {
      setResolvingApprovals((current) => {
        const next = new Set(current)
        next.delete(requestId)
        return next
      })
    }
  }, [flushState])

  const cards = cardOrderRef.current
    .filter((id) => activities.has(id))
    .map((id) => activities.get(id)!)
    .filter((card) => card.events.length > 0)

  return (
    <div
      className="flex flex-col bg-raised rounded-[var(--radius-md)] overflow-hidden border border-line-subtle"
      style={{ height }}
    >
      {/* Header（可拖动弹窗的把手） */}
      <div
        onPointerDown={onHeaderPointerDown}
        onDoubleClick={onHeaderDoubleClick}
        title={onHeaderPointerDown ? t('拖动标题栏移动，双击复位') : undefined}
        className={`flex items-center gap-2 px-3.5 py-2.5 border-b border-line-subtle shrink-0 select-none [touch-action:none] ${onHeaderPointerDown ? 'cursor-grab' : 'cursor-default'}`}
      >
        <Activity size={14} strokeWidth={2.2} className="text-accent shrink-0" />
        <span className="text-md font-semibold text-primary">
          {t('MCP 活动')}
        </span>
        <span className="text-xs px-1.5 py-[2px] rounded-[var(--radius-sm)] bg-accent-dim text-accent">
          {cards.length}
        </span>
        <div className="flex-1" />
        {onClose && (
          <button
            onClick={onClose}
            onPointerDown={(e) => e.stopPropagation()}
            onDoubleClick={(e) => e.stopPropagation()}
            className="bg-transparent border-none text-secondary cursor-pointer text-[16px] px-1.5 py-[2px] rounded-[var(--radius-sm)] hover:bg-hover hover:text-primary transition-colors duration-[80ms]"
            title={t('关闭')}
          >
            ✕
          </button>
        )}
      </div>

      {/* Activity list */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto p-2"
      >
        {cards.length === 0 ? (
          <div className="text-center px-5 py-10 text-tertiary text-base">
            {t('外部 MCP（如 Claude Code）的操作会显示在这里')}
          </div>
        ) : (
          cards.map((card) => {
            const latest = card.events[card.events.length - 1]
            const first = card.events[0]
            const color = statusColors[latest.status] || '#888'
            const clientColor = clientColors[first.clientName] || '#888'
            const needsApproval = latest.status === 'approval_required' && !card.resolved
            const resolvingApproval = resolvingApprovals.has(first.requestId)
            const approvalError = approvalErrors.get(first.requestId)

            return (
              <div
                key={first.requestId}
                className={`mb-2 px-3 py-2.5 rounded-[var(--radius-md)] bg-overlay border ${needsApproval ? '' : 'border-line-subtle'}`}
                style={needsApproval ? { borderColor: color } : undefined}
              >
                {/* Card header */}
                <div className="flex items-center gap-1.5 mb-1.5">
                  <span
                    className="text-[10px] font-semibold px-1.5 py-[2px] rounded-[var(--radius-sm)] uppercase tracking-[0.5px]"
                    style={{ background: `${clientColor}22`, color: clientColor }}
                  >
                    {first.clientName || 'unknown'}
                  </span>
                  <span className="text-xs text-secondary">
                    {first.serverName || '—'}
                  </span>
                  <span className="text-[10px] text-tertiary">
                    {first.tool}
                  </span>
                  <div className="flex-1" />
                  <span className="text-[10px] text-tertiary">
                    {formatTime(latest.timestamp)}
                  </span>
                </div>

                {/* Command display */}
                {first.command && (
                  <div className="text-xs font-mono text-secondary bg-black/25 px-2 py-1 rounded-[var(--radius-sm)] mb-1 whitespace-pre-wrap break-all">
                    {first.cwd ? `$ cd ${first.cwd}\n` : ''}$ {first.command}
                  </div>
                )}

                {/* Purpose */}
                {first.purpose && (
                  <div className="text-xs text-tertiary mb-1">
                    {first.purpose}
                  </div>
                )}

                {/* Status badge */}
                <div className="flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{
                    background: color,
                    animation: (latest.status === 'running' || latest.status === 'queued') ? 'pulse 1.5s ease-in-out infinite' : 'none',
                  }} />
                  <span className="text-xs font-medium" style={{ color }}>
                    {t((statusLabels[latest.status] || latest.status) as I18nKey)}
                    {latest.exitCode != null ? ` (exit ${latest.exitCode})` : ''}
                  </span>
                </div>

                {/* Output preview */}
                {latest.output && latest.output.trim() && (
                  <details className="mt-1.5">
                    <summary className="text-[10px] text-tertiary cursor-pointer">
                      {t('输出预览')}
                    </summary>
                    <pre className="text-[10px] font-mono text-secondary bg-black/30 px-2 py-1.5 rounded-[var(--radius-sm)] mt-1 max-h-[200px] overflow-auto whitespace-pre-wrap break-all">
                      {latest.output}
                    </pre>
                  </details>
                )}

                {/* Approval buttons */}
                {needsApproval && (
                  <div className="mt-2">
                    {approvalError && (
                      <div className="mb-1.5 text-[10px] text-danger break-all">{approvalError}</div>
                    )}
                    <div className="flex gap-2">
                      <Button
                        variant="success"
                        size="sm"
                        block
                        disabled={resolvingApproval}
                        onClick={() => { void handleResolveApproval(first.requestId, true, card) }}
                      >
                        {t('批准')}
                      </Button>
                      <Button
                        variant="danger"
                        size="sm"
                        block
                        disabled={resolvingApproval}
                        onClick={() => { void handleResolveApproval(first.requestId, false, card) }}
                      >
                        {t('拒绝')}
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}
