import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const layoutSource = readFileSync('src/routes/+layout.svelte', 'utf8');
const headerSource = readFileSync('src/lib/components/header.svelte', 'utf8');
const loginSource = readFileSync('src/routes/login/+page.svelte', 'utf8');
const settingsSource = readFileSync('src/routes/settings/+page.svelte', 'utf8');
const appDetailsSource = readFileSync('src/routes/apps/[id]/+page.svelte', 'utf8');
const deployDetailsSource = readFileSync('src/routes/deploys/[id]/+page.svelte', 'utf8');

test('layout composes header and footer shell', () => {
  assert.equal(layoutSource.includes('<Header />'), true, 'layout should render Header');
  assert.equal(layoutSource.includes('{#if isAuthenticated}'), true, 'layout should gate header by auth state');
  assert.equal(layoutSource.includes('<Footer />'), false, 'layout should not render Footer');
});

test('header exposes sprint 1 nav and login controls', () => {
  assert.equal(headerSource.includes('/dashboard'), true, 'header should link to dashboard');
  assert.equal(headerSource.includes('/settings'), true, 'header should link to settings');
  assert.equal(headerSource.includes('{#if isAuthenticated}'), true, 'header nav should only show when authenticated');
  assert.equal(headerSource.includes('<GithubLoginButton />'), false, 'header should not render GitHub login button');
});

test('sprint 2 auth and aws settings UI exists', () => {
  assert.equal(loginSource.includes('Create Session'), true, 'login page should create auth session');
  assert.equal(settingsSource.includes('Validate + Save'), true, 'settings page should save aws connection');
});

test('sprint 3 app details includes infra output and config history sections', () => {
  assert.equal(appDetailsSource.includes('Infra Outputs'), true, 'app details should show infra outputs');
  assert.equal(appDetailsSource.includes('Config History'), true, 'app details should show config history');
});

test('sprint 4 deploy controls and auto-deploy UX exist', () => {
  assert.equal(appDetailsSource.includes('Deploy Now'), true, 'app details should expose manual deploy action');
  assert.equal(appDetailsSource.includes('Auto-Deploy'), true, 'app details should show auto-deploy status');
  assert.equal(deployDetailsSource.includes('Cancel'), true, 'deploy details should expose cancel action');
  assert.equal(deployDetailsSource.includes('Retry'), true, 'deploy details should expose retry action');
});

test('sprint 5 AI insight UX is visible on deployment details', () => {
  assert.equal(deployDetailsSource.includes('AI Insight'), true, 'deploy details should include AI insight section');
  assert.equal(deployDetailsSource.includes('Generate AI Insight'), true, 'deploy details should include AI generation control');
  assert.equal(deployDetailsSource.includes('Bypass AI (Fallback)'), true, 'deploy details should support AI bypass');
  assert.equal(deployDetailsSource.includes('Recent AI Requests'), true, 'deploy details should show AI request history');
});
