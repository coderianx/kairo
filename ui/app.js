const body = document.body;

const getStartedButton = document.querySelector('.welcome__cta');
const providerCards = Array.from(
	document.querySelectorAll('.providers__grid .provider-card'),
);
const continueButton = document.querySelector('.providers__continue');

const heroLogo = document.querySelector('.provider-hero__logo');
const heroName = document.querySelector('.provider-hero__name');
const apiKeyInput = document.querySelector('#api-key');
const applyButton = document.querySelector('.connect__apply');
const apiKeyError = document.querySelector('.connect__error');

const settingsButton = document.querySelector('.sidebar__settings');
const brandHome = document.querySelector('.sidebar__brand');
const settingsGrid = document.querySelector('.settings__grid');
const connectedList = document.querySelector('.connected');
const settingsConnect = document.querySelector('.settings__connect');
const settingsInput = document.querySelector('#settings-api-key');
const settingsApply = document.querySelector('.settings-apply');
const settingsError = document.querySelector('.settings-error');

let selectedCard = null;
let selectedSettingsCard = null;

// Show exactly one app state at a time; no class means the welcome screen.
function showScreen(state) {
	body.classList.remove(
		'screen-providers',
		'screen-apikey',
		'screen-dashboard',
		'screen-settings',
	);

	if (state) {
		body.classList.add(state);
	}
}

// On boot, go straight to the dashboard when a provider is already
// connected in the backend database.
(async () => {
	try {
		const result = await window.kairo.listProviders();

		if (result.ok && result.providers.length > 0) {
			showScreen('screen-dashboard');
		}
	} catch {
		// IPC unavailable; stay on the welcome screen.
	}
})();

// Move the brand to the top-left corner and reveal the provider picker.
getStartedButton.addEventListener('click', () => {
	showScreen('screen-providers');

	const firstCard = providerCards[0];

	if (firstCard) {
		firstCard.focus();
	}
});

// Single-select behavior: exactly one card can be selected at a time.
// Selecting a card enables the continue button.
providerCards.forEach((card) => {
	card.addEventListener('click', () => {
		selectedCard = card;

		providerCards.forEach((other) => {
			const selected = other === card;

			other.classList.toggle('selected', selected);
			other.setAttribute('aria-pressed', String(selected));
		});

		continueButton.disabled = false;
	});
});

// Show the API key screen for the selected provider.
continueButton.addEventListener('click', () => {
	if (!selectedCard) {
		return;
	}

	const cardLogo = selectedCard.querySelector('.provider-card__logo');

	heroLogo.replaceChildren(
		cardLogo.querySelector('svg').cloneNode(true),
	);
	heroLogo.style.color = getComputedStyle(cardLogo).color;

	heroName.textContent = selectedCard.querySelector(
		'.provider-card__name',
	).textContent;

	showScreen('screen-apikey');

	apiKeyInput.focus();
});

// The apply button only appears once a key has been typed.
apiKeyInput.addEventListener('input', () => {
	applyButton.hidden = apiKeyInput.value.trim() === '';
	apiKeyError.hidden = true;
});

// Send the key to the backend through IPC; on success go to the dashboard.
applyButton.addEventListener('click', async () => {
	const apiKey = apiKeyInput.value.trim();

	if (!apiKey || !selectedCard || applyButton.disabled) {
		return;
	}

	applyButton.disabled = true;
	apiKeyInput.disabled = true;

	let result = { ok: false };

	try {
		result = await window.kairo.connectProvider({
			provider: selectedCard.dataset.provider,
			apiKey,
		});
	} catch {
		result = { ok: false, error: 'Connection failed. Try again.' };
	}

	if (result.ok) {
		showScreen('screen-dashboard');
	} else {
		apiKeyError.textContent =
			result.error || 'Could not connect. Check the key and try again.';
		apiKeyError.hidden = false;
	}

	applyButton.disabled = false;
	apiKeyInput.disabled = false;
});

// Settings: clone the provider cards from the onboarding screen so the
// markup stays defined in a single place.
Array.from(providerCards).forEach((card) => {
	const clone = card.cloneNode(true);

	clone.classList.remove('selected');
	clone.setAttribute('aria-pressed', 'false');

	settingsGrid.appendChild(clone);
});

const settingsCards = Array.from(
	settingsGrid.querySelectorAll('.provider-card'),
);

// Render the connected providers list from the backend.
function renderConnectedItem(provider) {
	const item = document.createElement('li');
	item.className = 'connected__item';

	const logo = document.createElement('span');
	logo.className = 'connected__logo';

	const sourceCard = document.querySelector(
		`.providers__grid .provider-card[data-provider="${provider.name}"] .provider-card__logo`,
	);

	if (sourceCard) {
		logo.style.color = getComputedStyle(sourceCard).color;
		logo.replaceChildren(
			sourceCard.querySelector('svg').cloneNode(true),
		);
	} else {
		logo.classList.add('connected__logo--generic');
		logo.textContent = provider.name.charAt(0).toUpperCase();
	}

	const name = document.createElement('span');
	name.className = 'connected__name';
	name.textContent = provider.name;

	const url = document.createElement('span');
	url.className = 'connected__url';
	url.textContent = provider.base_url;

	item.append(logo, name, url);

	return item;
}

async function loadConnectedProviders() {
	let result = { ok: false, providers: [] };

	try {
		result = await window.kairo.listProviders();
	} catch {
		// Rendered as the empty state below.
	}

	const providers = result.ok ? result.providers : [];

	if (providers.length === 0) {
		const empty = document.createElement('li');

		empty.className = 'connected__empty';
		empty.textContent = result.ok
			? 'No providers connected yet.'
			: 'Could not load connected providers.';

		connectedList.replaceChildren(empty);

		return;
	}

	connectedList.replaceChildren(...providers.map(renderConnectedItem));
}

// Open settings from the sidebar; refresh the connected list each time.
settingsButton.addEventListener('click', () => {
	showScreen('screen-settings');

	loadConnectedProviders();
});

// The sidebar brand acts as the home button.
brandHome.addEventListener('click', () => {
	showScreen('screen-dashboard');
});

// Selecting a card in settings reveals the inline connect form.
settingsCards.forEach((card) => {
	card.addEventListener('click', () => {
		selectedSettingsCard = card;

		settingsCards.forEach((other) => {
			const selected = other === card;

			other.classList.toggle('selected', selected);
			other.setAttribute('aria-pressed', String(selected));
		});

		settingsConnect.hidden = false;
		settingsError.hidden = true;

		settingsInput.focus();
	});
});

settingsInput.addEventListener('input', () => {
	settingsApply.hidden = settingsInput.value.trim() === '';
	settingsError.hidden = true;
});

// Connect a new provider from settings; on success clear the form and
// refresh the connected list.
settingsApply.addEventListener('click', async () => {
	const apiKey = settingsInput.value.trim();

	if (!apiKey || !selectedSettingsCard || settingsApply.disabled) {
		return;
	}

	settingsApply.disabled = true;
	settingsInput.disabled = true;

	let result = { ok: false };

	try {
		result = await window.kairo.connectProvider({
			provider: selectedSettingsCard.dataset.provider,
			apiKey,
		});
	} catch {
		result = { ok: false, error: 'Connection failed. Try again.' };
	}

	if (result.ok) {
		settingsInput.value = '';
		settingsConnect.hidden = true;

		selectedSettingsCard.classList.remove('selected');
		selectedSettingsCard.setAttribute('aria-pressed', 'false');
		selectedSettingsCard = null;

		await loadConnectedProviders();
	} else {
		settingsError.textContent =
			result.error || 'Could not connect. Check the key and try again.';
		settingsError.hidden = false;
	}

	settingsApply.disabled = false;
	settingsInput.disabled = false;
});
