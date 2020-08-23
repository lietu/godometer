<script>
  import { convertSpeed, convertDistance, speedDisplayUnits } from "./Data"

  export let mode
  export let distanceUnits
  export let speedUnits
  export let period
  export let data

  let periodText = period.slice(0, -1)
  const empty = { m: 0, mps: 0, kph: 0 }

  $: speedDisplay = speedDisplayUnits[$speedUnits]
  let currentData
  let relevant
  let earlier
  let now
  let earlierDistance
  let earlierSpeed
  let nowDistance
  let nowSpeed

  data.subscribe((current) => {
    currentData = current

    relevant = currentData === undefined ? [empty, empty] : currentData.slice(-2)
    earlier = relevant[0]
    now = relevant[1]

    earlierDistance = convertDistance(earlier.m, $distanceUnits)
    earlierSpeed = convertSpeed(earlier, $speedUnits)

    nowDistance = convertDistance(now.m, $distanceUnits)
    nowSpeed = convertSpeed(now, $speedUnits)
  })
</script>

<div class={"latest-data " + mode}>
  <h2 class="left now">Latest: {nowDistance} {$distanceUnits}
    @ {nowSpeed} {speedDisplay}</h2>
  <h2 class="right">Previous: {earlierDistance} {$distanceUnits}
    @ {earlierSpeed} {speedDisplay}</h2>
</div>

<style type="scss">
  .latest-data {
    display: flex;

    h2 {
      flex: 1;
      display: inline-block;

      &.now {
        font-weight: bold;
      }

      &.left {
        margin-left: 4px;
        text-align: left;
      }

      &.right {
        margin-right: 4px;
        text-align: right;
      }
    }
  }
</style>
