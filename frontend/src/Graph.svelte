<script>
  import { onMount, onDestroy } from "svelte"
  import chroma from "chroma-js"
  import Chart from "chart.js"
  import { Wave } from "svelte-loading-spinners"
  import {
    convertSpeed,
    convertDistance,
    convertLabel,
    speedDisplayUnits
  } from "./Data"

  export let mode
  export let distanceUnits
  export let speedUnits
  export let period
  export let data

  let containerWidth = 320
  let containerHeight = 200

  const periodColor = {
    minutes: chroma.hsl(171, 1, 0.41),
    hours: chroma.hsl(141, 0.71, 0.48),
    days: chroma.hsl(204, 0.86, 0.53),
    weeks: chroma.hsl(217, 0.71, 0.53),
    months: chroma.hsl(48, 1, 0.67),
    years: chroma.hsl(348, 1, 0.61),
  }

  const titles = {
    "minutes": "Last 60 minutes",
    "hours": "Last 24 hours",
    "days": "Last 7 days",
    "weeks": "Last 5 weeks",
    "months": "Last 12 months",
    "years": "Last 4 years"
  }

  let currentData
  let canvas
  let ctx
  let chart

  let chartData = {}
  let chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    legend: {
      position: "bottom",
      labels: {}
    },
    title: {
      display: true,
      text: titles[period]
    },
    scales: {
      xAxes: [
        {
          id: "x",
        }
      ],
      yAxes: [
        {
          id: "Distance",
          type: "linear",
          display: true,
          position: "left",
          suggestedMin: 0,
          beginAtZero: true
        },
        {
          id: "Speed",
          type: "linear",
          display: true,
          position: "right",
          suggestedMin: 0,
          beginAtZero: true
        },
      ]
    }
  }

  function updateChartData() {
    if (ctx === undefined) {
      return
    }

    const labels = []
    const distance = []
    const speed = []
    let maxDistance = 5
    let maxSpeed = 3

    const lineColor = periodColor[period]
    const gridColor = $mode === "dark" ? "rgba(255, 255, 255, 0.1)" : "rgba(0, 0, 0, 0.1)"
    const fontColor = $mode === "dark" ? "#aaa" : "#333"

    const distanceColor = lineColor.saturate(2).alpha(0.6).css()
    const speedColor = lineColor.desaturate(2).alpha(0.6).css()

    chartOptions.title.fontColor = fontColor
    chartOptions.legend.labels.fontColor = fontColor

    const ticks = {
      fontColor: fontColor
    }

    const gridLines = {
      color: gridColor
    }

    if (currentData !== undefined) {
      currentData.forEach((i) => {
        const d = convertDistance(i.m, $distanceUnits)
        const s = convertSpeed(i, $speedUnits)

        if (d > maxDistance) {
          maxDistance = d
        }

        if (s > maxSpeed) {
          maxSpeed = s
        }

        labels.push(convertLabel(i.ts, period))
        distance.push(d)
        speed.push(s)
      })

      chartData = {
        labels: labels,
        datasets: [
          {
            label: `Distance (${$distanceUnits})`,
            yAxisId: "Distance",
            borderColor: distanceColor,
            backgroundColor: distanceColor,
            fill: false,
            data: distance,
          },
          {
            label: `Speed (${speedDisplayUnits[$speedUnits]})`,
            xAxisId: "x",
            yAxisId: "Speed",
            borderColor: speedColor,
            backgroundColor: speedColor,
            fill: false,
            data: speed,
          }
        ],
      }

      chartOptions.scales.xAxes[0].ticks = ticks
      chartOptions.scales.yAxes[0].ticks = ticks
      chartOptions.scales.yAxes[1].ticks = ticks

      chartOptions.scales.xAxes[0].gridLines = gridLines
      chartOptions.scales.yAxes[0].gridLines = gridLines
      chartOptions.scales.yAxes[1].gridLines = gridLines

      chartOptions.scales.yAxes[0].suggestedMax = maxDistance
      chartOptions.scales.yAxes[1].suggestedMax = maxSpeed

      if (period === "minutes") {
        chartData.datasets[0].pointRadius = 1.5
        chartData.datasets[1].pointRadius = 1.5
      }

      if (chart === undefined) {
        chart = new Chart.Line(ctx, {
          type: "line",
          data: chartData,
          options: chartOptions,
        })
      } else {
        chart.data = chartData
        chart.options = chartOptions
        chart.update({
          duration: 0
        })
      }
    }
  }

  data.subscribe(function (update) {
    currentData = update
    updateChartData()
  })

  speedUnits.subscribe(() => {
    updateChartData()
  })

  distanceUnits.subscribe(() => {
    updateChartData()
  })

  mode.subscribe(() => {
    updateChartData()
  })

  onMount(() => {
    ctx = canvas.getContext("2d")
    updateChartData()
  })

  onDestroy(() => {
    if (chart) {
      chart.destroy()
    }
  })

  $: loaded = $data !== undefined
  $: width = Math.max(containerWidth, 128)
  $: height = Math.max(containerHeight, 128)
</script>

<div bind:clientWidth={containerWidth} bind:clientHeight={containerHeight}
     class={"graph-container " + period}>
  <div class="graph"
       style="width: {width}px; height: {height}px;">
    <canvas bind:this={canvas}></canvas>
  </div>
  {#if !loaded}
    <div class="graph-loader-container">
      <div class="graph-loader">
        <Wave size="8" color="#FF3E00" unit="rem" />
      </div>
    </div>
  {/if}
</div>

<style type="scss">
  .graph-loader-container {
    position: absolute;
    height: 100%;
    width: 100%;
    top: 0;
    left: 0;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .graph-container {
    position: relative;
    flex: 1;
    padding: 0;
    background: rgba(0, 0, 0, 0.01);
    border-radius: 5px;

    &.minutes {
      border: 3px solid rgba(0, 209, 178, 0.5);
    }

    &.hours {
      border: 3px solid rgba(72, 199, 116, 0.5);
    }

    &.days {
      border: 3px solid rgba(32, 156, 238, 0.5);
    }

    &.weeks {
      border: 3px solid rgba(50, 115, 220, 0.5);
    }

    &.months {
      border: 3px solid rgba(255, 221, 87, 0.5);
    }

    &.years {
      border: 3px solid rgba(255, 56, 96, 0.5);
    }
  }

  .graph {
    overflow: hidden;
    max-height: 33vh;
    position: absolute;
  }

  @media only screen and (max-width: 768px) {
    .graph-container {
      height: 16rem;
    }
  }
</style>
