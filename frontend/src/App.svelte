<script>
  import { writable } from "svelte/store"
  import "bulma/css/bulma.css"
  import { Button } from "svelma"
  import Graph from "./Graph.svelte"
  import {
    speedDisplayUnits,
    minutes,
    hours,
    days,
    weeks,
    months,
    years
  } from "./Data"
  import Latest from "./Latest.svelte";

  let speedUnits = writable("mps")
  let distanceUnits = writable("km")
  let mode = writable("light")

  const location = new URL(window.location)
  const urlSpeed = location.searchParams.get("speed")
  const urlDistance = location.searchParams.get("distance")
  const urlMode = location.searchParams.get("mode")

  if (urlSpeed) {
    speedUnits.set(urlSpeed)
  }

  if (urlDistance) {
    distanceUnits.set(urlDistance)
  }

  if (urlMode) {
    mode.set(urlMode)
  }

  function setSpeed(unit) {
    speedUnits.set(unit)
    location.searchParams.set("speed", unit)
    window.history.pushState({}, '', location.toString())
  }

  function setDistance(unit) {
    distanceUnits.set(unit)
    location.searchParams.set("distance", unit)
    window.history.pushState({}, '', location.toString())
  }

  function updateMode() {
    document.body.classList.remove("light", "dark")
    document.body.classList.add($mode)
  }

  function toggleMode() {
    $mode = $mode === "dark" ? "light" : "dark"
    location.searchParams.set("mode", $mode)
    window.history.pushState({}, '', location.toString())

    updateMode()
  }

  const speedUnitKeys = Object.keys(speedDisplayUnits)
  const distanceUnitKeys = ["m", "ft", "km", "mi"]
  const graphProps = {
    speedUnits,
    distanceUnits,
    mode,
  }

  updateMode()
</script>

<div class="flex-container">
  <h1>Godometer stats</h1>
  <div class="flex-row">
    <div class="flex-column">
      <Latest {...graphProps} period="minutes" data={minutes} />
      <Graph {...graphProps} period="minutes" data={minutes} />
    </div>
    <div class="flex-column">
      <Latest {...graphProps} period="hours" data={hours} />
      <Graph {...graphProps} period="hours" data={hours} />
    </div>
  </div>
  <div class="flex-row">
    <div class="flex-column">
      <Latest {...graphProps} period="days" data={days} />
      <Graph {...graphProps} period="days" data={days} />
    </div>
    <div class="flex-column">
      <Latest {...graphProps} period="weeks" data={weeks} />
      <Graph {...graphProps} period="weeks" data={weeks} />
    </div>
  </div>
  <div class="flex-row">
    <div class="flex-column">
      <Latest {...graphProps} period="months" data={months} />
      <Graph {...graphProps} period="months" data={months} />
    </div>
    <div class="flex-column">
      <Latest {...graphProps} period="years" data={years} />
      <Graph {...graphProps} period="years" data={years} />
    </div>
  </div>
</div>

<div class="unit-selector">
  <div class="button-list">
    <Button class={"capitalize is-small " + ($mode === "dark" ? "is-dark" : "is-light")}
            on:click={() => toggleMode()}>{$mode}</Button>
    {#each speedUnitKeys as key}
      <Button class={"is-small " + (key === $speedUnits ? "is-outlined" : "is-primary")}
              on:click={() => setSpeed(key)}>{speedDisplayUnits[key]}</Button>
    {/each}
    {#each distanceUnitKeys as key}
      <Button class={"is-small " + (key === $distanceUnits ? "is-outlined" : "is-info")}
              on:click={() => setDistance(key)}>{key}</Button>
    {/each}
  </div>
</div>

<style type="scss">
  $graphMargins: 0.5rem;
  $buttonMargins: 2px;

  :global(html, body) {
    width: 100%;
    height: 100%;
    display: flex;
    margin: 0;
    padding: 0;
    overflow: hidden;
    overflow-y: hidden !important;

    &.dark {
      background: #090909;
      color: #fff;
    }
  }

  h1 {
    text-align: left;
    height: 32px;
    font-weight: bold;
    font-size: 1.5rem;
    margin-bottom: 1rem;
  }

  .unit-selector {
    position: absolute;
    right: 0;
    top: 0;

    .button-list {
      text-align: right;
      margin-top: $buttonMargins;

      :global(button) {
        margin-left: $buttonMargins;

        &:last-child {
          margin-right: $buttonMargins;
        }
      }
    }
  }

  .flex-container {
    display: flex;
    flex: 1;
    flex-direction: column;
  }

  .flex-column {
    height: 100%;
    display: flex;
    flex-direction: column;
    flex: 1;
    margin-left: $graphMargins;
  }

  .flex-column:last-child {
    margin-right: $graphMargins;
  }

  .flex-row {
    width: 100%;
    flex-direction: row;
    display: flex;
    flex: 1;
    margin-top: $graphMargins;
  }

  .flex-row:last-child {
    margin-bottom: $graphMargins;
  }

  @media only screen and (max-width: 768px) {
    :global(body) {
      overflow: scroll;
      overflow-y: scroll !important;
    }

    .flex-container, .flex-column, .flex-row {
      display: block;
    }

    .flex-column {
      margin: $graphMargins;
    }

    .flex-row {
      margin: 0;
    }
  }

</style>
