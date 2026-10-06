// Onboarding wizard steps 2 and 3, which no route preset can reach — the `onboarding`
// preset stops at step 1 because that is where a fresh state lands.
//
//   node .kilo/skills/ui-capture/scripts/uicapture.mjs \
//     --script .kilo/skills/ui-capture/scripts/flows/onboarding-steps.mjs
//
// The demo user's OnboardingState has nothing marked done, so restoreState()
// shows step 1. Clicking through mirrors a real signup; the profile step writes
// the display name, which is why the flow stops there instead of finishing.

export default async function flow({ page, shot, goto, settle }) {
  await goto('/web/onboarding');
  await settle('#step1:not(.d-none)');
  await shot('onboarding-step-1-profile');

  await page.click('#btnSaveProfile');
  await settle('#step2:not(.d-none)');
  await shot('onboarding-step-2-household');

  // Browse mode lists the public households; Create and Invite code are the
  // other two sub-modes of the same step.
  await page.click('#btnHouseholdModeCreate');
  await settle('#householdModeCreate:not(.d-none)');
  await shot('onboarding-step-2-create-household');

  await page.click('#btnHouseholdModeInvite');
  await settle('#householdModeInvite:not(.d-none)');
  await shot('onboarding-step-2-invite-code');

  // btnNextStep advances without changing any state, so step 3 is reachable
  // without joining or creating a household.
  await page.click('#btnNextStep');
  await settle('#step3:not(.d-none)');
  await shot('onboarding-step-3-notifications');
}