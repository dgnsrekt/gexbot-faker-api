import { describe, it, expect } from 'vitest'
import { promToSeries, type PromResponse } from './api'

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
