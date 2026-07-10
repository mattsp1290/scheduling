declare global {
	type User = { id: number; email: string; name: string };
	type TimeSlot = { id?: number; start: string; end: string };
	type Survey = { id: number; title: string; description: string; timezone: string; share_token: string; created_by?: number; slots: TimeSlot[] };
	type ResponseResult = { id: number; respondent_name: string; slot_ids: number[] };
	type SurveyResults = { survey: Survey; responses: ResponseResult[]; slot_counts: Record<string, number>; respondents: Record<string, string[]> };
}

export {};
