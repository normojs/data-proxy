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
import assert from 'node:assert/strict'
import {
  normalizeMCPProxyCallHealth,
  normalizeMCPProxyTrendResponse,
} from '../src/features/mcp/lib/proxy-trends.ts'

const emptyHealth = normalizeMCPProxyCallHealth(undefined)
assert.deepEqual(emptyHealth, {
  total_calls: 0,
  success_calls: 0,
  error_calls: 0,
  timeout_calls: 0,
  pending_calls: 0,
  settled_calls: 0,
  unsettled: 0,
  free_calls: 0,
  quota: 0,
  cost: 0,
  result_size: 0,
  avg_duration_ms: 0,
  success_rate: 0,
})

const emptyTrend = normalizeMCPProxyTrendResponse({})
assert.deepEqual(emptyTrend.buckets, [])
assert.deepEqual(emptyTrend.servers, [])
assert.deepEqual(emptyTrend.tools, [])
assert.equal(emptyTrend.totals.total_calls, 0)
assert.equal(emptyTrend.start_time, 0)
assert.equal(emptyTrend.end_time, 0)

const partialTrend = normalizeMCPProxyTrendResponse({
  start_time: 100,
  end_time: 200,
  totals: {
    total_calls: 4,
    success_calls: 3,
    error_calls: 1,
  },
  buckets: [
    null,
    {
      bucket_start: 100,
      total_calls: 4,
      success_calls: 2,
      timeout_calls: 1,
    },
  ],
  servers: [
    {
      proxy_server_id: 7,
      name: 'primary',
      total_calls: 2,
    },
  ],
  tools: [
    {
      proxy_tool_id: 11,
      exposed_tool_name: 'pet_api.getpet',
      total_calls: 1,
    },
  ],
})

assert.equal(partialTrend.totals.success_rate, 75)
assert.equal(partialTrend.buckets.length, 1)
assert.equal(partialTrend.buckets[0].success_rate, 50)
assert.equal(partialTrend.buckets[0].error_calls, 0)
assert.equal(partialTrend.buckets[0].timeout_calls, 1)
assert.equal(partialTrend.servers.length, 1)
assert.equal(partialTrend.servers[0].namespace, '')
assert.equal(partialTrend.servers[0].success_rate, 0)
assert.equal(partialTrend.tools.length, 1)
assert.equal(partialTrend.tools[0].proxy_server_id, 0)
assert.equal(partialTrend.tools[0].downstream_tool_name, '')

console.log('MCP trend smoke passed: empty and partial responses normalize')
