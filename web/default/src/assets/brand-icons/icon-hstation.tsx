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
import { type SVGProps } from 'react'
import { cn } from '@/lib/utils'

export function IconHStation({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      role='img'
      viewBox='0 0 16 16'
      xmlns='http://www.w3.org/2000/svg'
      width='16'
      height='16'
      className={cn(className)}
      {...props}
    >
      <title>H Station</title>
      <rect width='16' height='16' rx='3.2' fill='#111827' />
      <path
        d='M4.25 3.8h1.8v3.35h3.9V3.8h1.8v8.4h-1.8V8.75h-3.9v3.45h-1.8V3.8Z'
        fill='#f9fafb'
      />
      <path d='M2.65 13.15h10.7' stroke='#facc15' strokeWidth='1.1' />
    </svg>
  )
}
