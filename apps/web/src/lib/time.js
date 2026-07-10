export function formatSlot(slot) {
	const start = new Date(slot.start);
	const end = new Date(slot.end);
	return `${start.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' })} ${start.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}–${end.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}`;
}

export function toISO(date, hour) {
	const value = new Date(date);
	value.setHours(hour, 0, 0, 0);
	return value.toISOString();
}
