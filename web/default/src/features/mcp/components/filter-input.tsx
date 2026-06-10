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
import type { Table } from '@tanstack/react-table'
import { Input } from '@/components/ui/input'

type FilterInputProps<TData> = {
  className?: string
  columnId: string
  placeholder: string
  table: Table<TData>
}

export function FilterInput<TData>(props: FilterInputProps<TData>) {
  return (
    <Input
      placeholder={props.placeholder}
      value={
        (props.table.getColumn(props.columnId)?.getFilterValue() as string) ??
        ''
      }
      onChange={(event) =>
        props.table
          .getColumn(props.columnId)
          ?.setFilterValue(event.target.value)
      }
      className={props.className ?? 'w-full sm:w-[180px]'}
    />
  )
}
