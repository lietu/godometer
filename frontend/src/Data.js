import { writable } from 'svelte/store'

const updateInterval = 15000
let poller = undefined

const keepEvents = 15
const periods = ['minutes', 'hours', 'days', 'weeks', 'months', 'years']
const loadedPeriods = []
const neededPeriods = periods.length

const periodEvents = {
  minutes: [],
  hours: [],
  days: [],
  weeks: [],
  months: [],
  years: [],
}

const distanceConversions = {
  // Miles
  mi: {
    ratio: 0.0006213712,
  },
  // Kilometers
  km: {
    ratio: 0.001,
  },
  // Feet
  ft: {
    ratio: 3.28084,
  },
}

const speedConversions = {
  mph: {
    from: 'kph',
    ratio: 0.621371,
  },
  fps: {
    from: 'mps',
    ratio: 0.3048,
  },
  knots: {
    from: 'mps',
    ratio: 0.514444,
  },
}

const monthLabels = [
  'Jan',
  'Feb',
  'Mar',
  'Apr',
  'May',
  'Jun',
  'Jul',
  'Aug',
  'Sep',
  'Nov',
  'Oct',
  'Dec',
]

export const speedDisplayUnits = {
  mps: 'm/s',
  kph: 'km/h',
  mph: 'mph',
  fps: 'ft/s',
  knots: 'knots',
}

// Source: https://weeknumber.net/how-to/javascript
function dateISOWeek(src) {
  let date = new Date(src.getTime())
  date.setHours(0, 0, 0, 0)

  // Thursday in current week decides the year.
  date.setDate(date.getDate() + 3 - ((date.getDay() + 6) % 7))

  // January 4 is always in week 1.
  let week1 = new Date(date.getFullYear(), 0, 4)

  // Adjust to Thursday in week 1 and count number of weeks from date to week 1.
  return (
    1 +
    Math.round(
      ((date.getTime() - week1.getTime()) / 86400000 - 3 + ((week1.getDay() + 6) % 7)) /
        7
    )
  )
}

function tsToPeriodTs(ts, period) {
  if (period === 'minutes') {
    return ts
  } else if (period === 'hours') {
    return ts.substr(0, 13)
  } else if (period === 'days') {
    return ts.substr(0, 10)
  } else if (period === 'weeks') {
    const date = new Date(ts)
    return `${date.getUTCFullYear()} week ${dateISOWeek(date)}`
  } else if (period === 'months') {
    return ts.substr(0, 7)
  } else if (period === 'years') {
    return ts.substr(0, 4)
  }

  throw new Error(`Invalid timestamp period ${period}`)
}

function updatePeriodEvent(period, event) {
  // Save this event as processed
  const periodTs = tsToPeriodTs(event.ts, period)
  const processed = periodEvents[period].indexOf(periodTs) !== -1

  if (!processed) {
    periodEvents[period].push(periodTs)
    periodEvents[period] = periodEvents[period].splice(-keepEvents)
  }

  // Don't reprocess events except where it kinda may sometimes matter
  if (processed && period !== 'minutes') {
    return
  }

  const dataPoints = periodDataPoints[period]
  if (dataPoints === undefined || dataPoints.length === 0) {
    return
  }

  let found = false
  dataPoints.forEach((dp) => {
    if (dp.ts === periodTs) {
      if (period === 'minutes') {
        dp.m = event.m
      } else {
        dp.m += event.m
      }
      found = true
    }
  })

  // Not in the list yet, new data
  if (!found) {
    dataPoints.push({
      ts: periodTs,
      m: event.m,
      mps: event.mps,
      kph: event.kph,
    })
    dataPoints.shift()
  }

  periodStores[period].set(dataPoints)
}

async function pollEvents() {
  const apiUrl = `/api/v1/stats/events`
  const response = await fetch(apiUrl, {
    keepalive: true,
  })

  if (
    response.ok &&
    response.headers.get('Content-Type').startsWith('application/json')
  ) {
    const body = await response.json()
    const events = body.events
    events.forEach((event) => {
      periods.forEach((period) => {
        updatePeriodEvent(period, event)
      })
    })
  }
}

async function readStats(period) {
  const apiUrl = `/api/v1/stats/${period}`
  const response = await fetch(apiUrl, {
    keepalive: true,
  })

  if (
    response.ok &&
    response.headers.get('Content-Type').startsWith('application/json')
  ) {
    const data = await response.json()
    periodEvents[period] = data.eventTimestamps.splice(-keepEvents)
    periodStores[period].set(data.dataPoints)

    if (loadedPeriods.indexOf(period) === -1) {
      loadedPeriods.push(period)
    }

    if (loadedPeriods.length === neededPeriods && poller === undefined) {
      poller = setInterval(pollEvents, updateInterval)
    }
  }
}

function periodWritable(period, interval) {
  const w = writable(undefined)

  function handler() {
    readStats(period)
  }

  setTimeout(handler, 500 + Math.ceil(Math.random() * 10) * 50)
  setInterval(handler, interval)
  return w
}

export function convertLabel(ts, period) {
  const th = 'ᵗʰ'
  const nd = 'ⁿᵈ'
  const st = 'ˢᵗ'
  const rd = 'ʳᵈ'

  if (period === 'years') {
    return ts
  } else if (period === 'months') {
    return monthLabels[parseInt(ts.split('-').splice('-1'), 10) - 1]
  } else if (period === 'weeks') {
    return 'W' + ts.split(' ').slice(-1)
  } else if (period === 'days') {
    const parts = ts.split('-')
    const intMonth = parseInt(parts[1], 10)
    const intDay = parseInt(parts[2], 10)

    let dayStr = `${intDay}${th}`
    if (intDay < 10 || intDay > 20) {
      const lastDigit = parseInt(parts[2].slice(-1), 10)
      if (lastDigit === 1) {
        dayStr = `${intDay}${st}`
      } else if (lastDigit === 2) {
        dayStr = `${intDay}${nd}`
      } else if (lastDigit === 3) {
        dayStr = `${intDay}${rd}`
      }
    }

    return `${monthLabels[intMonth - 1]} ${dayStr}`
  } else if (period === 'minutes') {
    return ts.slice(-5)
  }

  return ts.slice(-2)
}

export function convertDistance(meters, target) {
  let value = meters
  if (target !== 'm') {
    const conversion = distanceConversions[target]
    value = conversion.ratio * meters
  }
  return value
}

function getDecimals(number) {
  let decimals = 0
  number = Math.abs(number)
  if (number > 0) {
    while (Math.round(number) === 0) {
      decimals += 1
      number *= 10
    }
  }
  return decimals
}

export function ceilDefault(value, defaultValue) {
  const decimals = Math.max(getDecimals(value), getDecimals(defaultValue))
  const multiplier = Math.pow(10, decimals)
  return Math.ceil(value * multiplier) / multiplier
}

export function round1(value) {
  const decimals = getDecimals(value) + 1
  const multiplier = Math.pow(10, decimals)
  return Math.round(value * multiplier) / multiplier
}

export function convertSpeed(data, target) {
  let value = data.mps
  if (target === 'kph') {
    value = data.kph
  } else if (target !== 'mps') {
    const conversion = speedConversions[target]
    if (conversion === undefined) {
      throw new Error(`Invalid conversion target ${target}`)
    }

    value = conversion.ratio * data[conversion.from]
  }
  return parseFloat(value.toFixed(1))
}

const minuteMs = 1000 * 60
export const minutes = periodWritable('minutes', minuteMs * 10)
export const hours = periodWritable('hours', minuteMs * 60)
export const days = periodWritable('days', minuteMs * 60 * 4)
export const weeks = periodWritable('weeks', minuteMs * 60 * 24)
export const months = periodWritable('months', minuteMs * 60 * 24 * 7)
export const years = periodWritable('years', minuteMs * 60 * 24 * 14)

const periodStores = {
  minutes: minutes,
  hours: hours,
  days: days,
  weeks: weeks,
  months: months,
  years: years,
}

const periodDataPoints = {
  minutes: undefined,
  hours: undefined,
  days: undefined,
  weeks: undefined,
  months: undefined,
  years: undefined,
}

Object.keys(periodStores).forEach((key) => {
  periodStores[key].subscribe((value) => {
    periodDataPoints[key] = value
  })
})

pollEvents()
