import { describe, it, expect, vi, afterEach } from 'vitest'
import { promToSeries, api, type PromResponse } from './api'

function range(values: [number, string][], metric: Record<string, string> = {}): PromResponse {
  return { status: 'success', data: { resultType: 'matrix', result: [{ metric, values }] } }
}

describe('promToSeries', () => {
  it('drops non-finite samples but keeps the surrounding valid points', () => {
    const r = range([
      [1, '10'],
      [2, 'NaN'], // Prometheus gap (e.g. histogram_quantile with no observations)
      [3, '+Inf'],
      [4, '20'],
    ])
    const s = promToSeries(r)
    expect(s).toHaveLength(1)
    expect(s[0].points).toEqual([
      [1, 10],
      [4, 20],
    ])
  })

  it('omits a series left with no finite points', () => {
    const r = range([
      [1, 'NaN'],
      [2, '+Inf'],
    ])
    expect(promToSeries(r)).toHaveLength(0)
  })

  it('labels series by the requested metric label', () => {
    const r: PromResponse = {
      status: 'success',
      data: {
        resultType: 'matrix',
        result: [
          { metric: { ticker: 'SPX' }, values: [[1, '17000']] },
          { metric: { ticker: 'NDX' }, values: [[1, '17100']] },
        ],
      },
    }
    expect(promToSeries(r, 'ticker').map((s) => s.name)).toEqual(['SPX', 'NDX'])
  })

  it('returns [] for an error response', () => {
    expect(promToSeries({ status: 'error', error: 'down' })).toEqual([])
  })
})

function jsonRes(data: unknown) {
  return { ok: true, json: async () => data, text: async () => JSON.stringify(data) }
}

describe('api.load', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('resolves once the job reaches done', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (path: string) =>
        path === '/load'
          ? jsonRes({ job_id: 'j1', state: 'queued' })
          : jsonRes({ job_id: 'j1', state: 'done', dates: ['2026-08-07'] }),
      ),
    )
    vi.useFakeTimers()
    const p = api.load('2026-08-07')
    await vi.advanceTimersByTimeAsync(1000)
    await expect(p).resolves.toMatchObject({ state: 'done' })
  })

  it('rejects when the job errors', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (path: string) =>
        path === '/load'
          ? jsonRes({ job_id: 'j1', state: 'queued' })
          : jsonRes({ job_id: 'j1', state: 'error', error: 'no archive' }),
      ),
    )
    vi.useFakeTimers()
    const p = api.load('2026-08-07')
    const rejects = expect(p).rejects.toThrow('no archive')
    await vi.advanceTimersByTimeAsync(1000)
    await rejects
  })

  // A job stuck queued past the poll budget is a timeout, not success — the Library Load button must
  // not clear its busy state as though the day loaded.
  it('rejects a job that never leaves queued (timeout, not success)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonRes({ job_id: 'j1', state: 'queued' })),
    )
    vi.useFakeTimers()
    const p = api.load('2026-08-07')
    const rejects = expect(p).rejects.toThrow(/did not finish/)
    await vi.advanceTimersByTimeAsync(240 * 500 + 1000)
    await rejects
  })
})

describe('api.loadRange', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  // The "Load span" control posts the range body and polls to done, so the loaded badges span it.
  it('posts the span body and resolves when the job is done', async () => {
    let posted: unknown
    vi.stubGlobal(
      'fetch',
      vi.fn(async (path: string, opts?: { body?: string }) => {
        if (path === '/load') {
          posted = JSON.parse(opts!.body!)
          return jsonRes({ job_id: 'r1', state: 'queued' })
        }
        return jsonRes({ job_id: 'r1', state: 'done', dates: ['2026-08-06', '2026-08-07', '2026-08-10'] })
      }),
    )
    vi.useFakeTimers()
    const p = api.loadRange({ from: '2026-08-06', to: '2026-08-10' })
    await vi.advanceTimersByTimeAsync(1000)
    const j = await p
    expect(posted).toEqual({ from: '2026-08-06', to: '2026-08-10' })
    expect(j.state).toBe('done')
    expect(j.dates).toHaveLength(3)
  })
})

describe('api.daemon', () => {
  afterEach(() => vi.restoreAllMocks())

  it('returns the proxied daemon status', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (path: string) => {
        expect(path).toBe('/studio/api/daemon')
        return jsonRes({ ready: true, config_path: '/app/configs/custom.yaml', tickers: ['SPX'] })
      }),
    )
    const d = await api.daemon()
    expect(d.ready).toBe(true)
    expect(d.config_path).toBe('/app/configs/custom.yaml')
  })

  it('rejects when the daemon proxy degrades (502/503)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({ ok: false, status: 502, json: async () => ({}), text: async () => 'bad gateway' })),
    )
    await expect(api.daemon()).rejects.toThrow()
  })
})

describe('api.download', () => {
  afterEach(() => vi.restoreAllMocks())

  // Coverage is YAML-authoritative: the Download screen must submit ONLY dates, never
  // tickers/packages, so a modified request can't change coverage.
  it('submits only dates (no ticker/package coverage)', async () => {
    let sentBody: unknown
    vi.stubGlobal(
      'fetch',
      vi.fn(async (path: string, opts: { body: string }) => {
        expect(path).toBe('/studio/api/download')
        sentBody = JSON.parse(opts.body)
        return jsonRes([])
      }),
    )
    await api.download(['2026-08-07', '2026-08-10'])
    expect(sentBody).toEqual({ dates: ['2026-08-07', '2026-08-10'] })
    expect(sentBody).not.toHaveProperty('tickers')
    expect(sentBody).not.toHaveProperty('packages')
  })
})
