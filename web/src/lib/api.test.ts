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
