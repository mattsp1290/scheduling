<script>
	import { toISO, formatSlot } from '$lib/time.js';

	export let slots = [];

	const hours = Array.from({ length: 13 }, (_, i) => i + 8);
	let selectedDayIndex = 0;
	let days = Array.from({ length: 14 }, (_, i) => {
		const date = new Date();
		date.setDate(date.getDate() + i);
		date.setHours(0, 0, 0, 0);
		return date;
	});

	function keyFor(date, hour) {
		return `${date.toDateString()}-${hour}`;
	}

	function isSelected(date, hour) {
		const start = toISO(date, hour);
		return slots.some((slot) => slot.start === start);
	}

	function toggle(date, hour) {
		const start = toISO(date, hour);
		const end = toISO(date, hour + 1);
		if (isSelected(date, hour)) {
			slots = slots.filter((slot) => slot.start !== start);
		} else {
			slots = [...slots, { start, end }].sort((a, b) => a.start.localeCompare(b.start));
		}
	}

	function removeSlot(start) {
		slots = slots.filter((slot) => slot.start !== start);
	}
</script>

<div class="calendar">
	<div class="day-tabs">
		{#each days as day, i}
			<button type="button" class:active={selectedDayIndex === i} on:click={() => (selectedDayIndex = i)}>
				{day.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' })}
			</button>
		{/each}
	</div>

	<div class="hour-grid">
		{#each hours as hour}
			<button type="button" class:selected={isSelected(days[selectedDayIndex], hour)} on:click={() => toggle(days[selectedDayIndex], hour)}>
				{new Date(toISO(days[selectedDayIndex], hour)).toLocaleTimeString([], { hour: 'numeric' })}
			</button>
		{/each}
	</div>

	{#if slots.length}
		<ul class="slot-list">
			{#each slots as slot (slot.start)}
				<li class="slot-pill">{formatSlot(slot)} <button type="button" class="link" on:click={() => removeSlot(slot.start)}>×</button></li>
			{/each}
		</ul>
	{:else}
		<p>Select one or more one-hour slots from the calendar.</p>
	{/if}
</div>
