import { expect, test } from '@playwright/test';

test('user creates a scheduling survey and a respondent submits availability', async ({ page }) => {
	const email = `e2e-${Date.now()}@example.com`;

	await page.goto('/signup');
	await expect(page.getByRole('navigation').getByRole('link', { name: 'Login' })).toBeVisible({ timeout: 15000 });
	await page.getByLabel('Name').fill('E2E Creator');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Password').fill('good-password');
	await page.getByRole('button', { name: 'Sign up' }).click();
	await page.waitForURL('/', { timeout: 15000 });
	await expect(page.getByRole('heading', { name: 'Your scheduling surveys' })).toBeVisible({ timeout: 15000 });
	await page.getByRole('link', { name: 'Create survey' }).click();

	await page.getByLabel('Title').fill('E2E planning session');
	await page.getByLabel('Description').fill('Pick the slot that works for you.');
	await page.locator('.hour-grid button').first().click();
	await page.getByRole('button', { name: 'Create share link' }).click();

	await expect(page.getByText('Survey created.')).toBeVisible();
	const shareHref = await page.getByRole('link', { name: /\/s\// }).first().getAttribute('href');
	expect(shareHref).toBeTruthy();

	await page.goto(shareHref);
	await expect(page.getByRole('heading', { name: 'E2E planning session' })).toBeVisible();
	await page.getByLabel('Your name').fill('Dana Respondent');
	await page.locator('button.secondary').first().click();
	await page.getByRole('button', { name: 'Submit availability' }).click();
	await expect(page.getByText('Thanks — your availability was saved.')).toBeVisible();

	const token = shareHref.split('/').pop();
	await page.goto(`/surveys/${token}/results`);
	await expect(page.getByRole('heading', { name: 'E2E planning session results' })).toBeVisible();
	await expect(page.getByText('Dana Respondent').first()).toBeVisible();
	await expect(page.getByText('1 available')).toBeVisible();
});
