const body = document.body;

const getStartedButton = document.querySelector('.welcome__cta');
const providerCards = Array.from(
	document.querySelectorAll('.providers__grid .provider-card'),
);
const continueButton = document.querySelector('.providers__continue');

const heroLogo = document.querySelector('.provider-hero__logo');
const heroName = document.querySelector('.provider-hero__name');
const apiKeyLabel = document.querySelector('label[for="api-key"]');
const apiKeyInput = document.querySelector('#api-key');
const applyButton = document.querySelector('.connect__apply');
const apiKeyError = document.querySelector('.connect__error');

const settingsButton = document.querySelector('.sidebar__settings');
const brandHome = document.querySelector('.sidebar__brand');
const settingsGrid = document.querySelector('.settings__grid');
const connectedList = document.querySelector('.connected');
const settingsConnect = document.querySelector('.settings__connect');
const settingsLabel = document.querySelector('label[for="settings-api-key"]');
const settingsInput = document.querySelector('#settings-api-key');
const settingsApply = document.querySelector('.settings-apply');
const settingsError = document.querySelector('.settings-error');

const newChatButton = document.querySelector('.sidebar__new-chat');
const chatListEl = document.querySelector('#chat-list');
const chatMessagesEl = document.querySelector('.chat__messages');
const chatColumn = document.querySelector('.chat__column');
const chatBlank = document.querySelector('.chat__blank');
const chatComposer = document.querySelector('#chat-composer');
const chatTextarea = document.querySelector('#chat-message');
const chatSendButton = document.querySelector('#chat-send');
const chatError = document.querySelector('.chat__error');
const providerPill = document.querySelector('#provider-pill');
const providerPillIcon = providerPill.querySelector('.pill__icon');
const providerPillLabel = providerPill.querySelector('.pill__label');
const providerMenu = document.querySelector('#provider-menu');
const modelPill = document.querySelector('#model-pill');
const modelPillLabel = modelPill.querySelector('.pill__label');
const modelMenu = document.querySelector('#model-menu');
const modelMenuList = document.querySelector('#model-menu-list');
const modelSearch = document.querySelector('#model-search');
const menuBackdrop = document.querySelector('#menu-backdrop');

let selectedCard = null;
let selectedSettingsCard = null;
let connectedProvidersCache = [];
let currentChatId = null;
let currentProviderName = '';
let currentModel = '';
let modelChoices = [];

// Local runtimes (Ollama) connect without an API key.
function isLocalProvider(name) {
	return name === 'ollama';
}

// Show exactly one app state at a time; no class means the welcome screen.
function showScreen(state) {
	body.classList.remove(
		'screen-providers',
		'screen-apikey',
		'screen-dashboard',
		'screen-settings',
		'screen-chat',
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

		connectedProvidersCache = result.ok ? result.providers : [];

		if (connectedProvidersCache.length > 0 && !currentProviderName) {
			currentProviderName = connectedProvidersCache[0].name;
		}

		renderProviderPill();
		renderModelPill();

		if (result.ok && result.providers.length > 0) {
			showScreen('screen-dashboard');
		}
	} catch {
		// IPC unavailable; stay on the welcome screen.
	}

	refreshChatList();
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

// Show the API key screen for the selected provider. Local providers
// (Ollama) skip the key input and connect straight away.
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

	const local = isLocalProvider(selectedCard.dataset.provider);

	// Local providers get an explanatory label instead of the key input.
	apiKeyLabel.textContent = local ? 'No API key required' : 'API key';
	apiKeyInput.hidden = local;
	apiKeyError.hidden = true;

	// Local providers can apply immediately; the others only once a key
	// has been typed.
	applyButton.hidden = !local;
	applyButton.textContent = local ? 'Connect' : 'Apply';

	showScreen('screen-apikey');

	(local ? applyButton : apiKeyInput).focus();
});

// The apply button only appears once a key has been typed.
apiKeyInput.addEventListener('input', () => {
	applyButton.hidden = apiKeyInput.value.trim() === '';
	apiKeyError.hidden = true;
});

// Send the key to the backend through IPC; on success go to the dashboard.
applyButton.addEventListener('click', async () => {
	const apiKey = apiKeyInput.value.trim();

	const local = isLocalProvider(selectedCard?.dataset.provider);

	if ((!apiKey && !local) || !selectedCard || applyButton.disabled) {
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
		await refreshProvidersCache();
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

const TRASH_SVG =
	'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 6h18"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/><line x1="10" x2="10" y1="11" y2="17"/><line x1="14" x2="14" y1="11" y2="17"/></svg>';

// Render the connected providers list from the backend.
function renderConnectedItem(provider) {
	const item = document.createElement('li');
	item.className = 'connected__item';
	item.dataset.providerId = String(provider.id);

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

	const del = document.createElement('button');
	del.type = 'button';
	del.className = 'connected__delete';
	del.setAttribute('aria-label', `Delete ${provider.name}`);
	del.title = 'Delete';
	del.innerHTML = TRASH_SVG;

	item.append(logo, name, url, del);

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

// Delete flow: the first click arms the button, the second click within the
// confirmation window performs the deletion. Delegated on the list so it
// survives re-renders.
let deleteArmTimer = null;

function disarmDelete(button) {
	button.classList.remove('armed');
	button.removeAttribute('data-armed');
	button.setAttribute(
		'aria-label',
		`Delete ${button.closest('.connected__item').querySelector('.connected__name').textContent}`,
	);
}

connectedList.addEventListener('click', async (event) => {
	const button = event.target.closest('.connected__delete');

	if (!button) {
		return;
	}

	if (!button.hasAttribute('data-armed')) {
		connectedList
			.querySelectorAll('.connected__delete[data-armed]')
			.forEach(disarmDelete);

		clearTimeout(deleteArmTimer);

		button.classList.add('armed');
		button.setAttribute('data-armed', '');
		button.setAttribute('aria-label', 'Confirm delete');

		deleteArmTimer = setTimeout(() => disarmDelete(button), 3500);

		return;
	}

	clearTimeout(deleteArmTimer);

	const itemId = Number(button.closest('.connected__item').dataset.providerId);

	try {
		await window.kairo.deleteProvider(itemId);
	} catch {
		// Refresh below shows the actual state either way.
	}

	await loadConnectedProviders();
});

// Open settings from the sidebar; refresh the connected list each time.
settingsButton.addEventListener('click', () => {
	showScreen('screen-settings');

	loadConnectedProviders();
});

// The sidebar brand acts as the home button.
brandHome.addEventListener('click', () => {
	showScreen('screen-dashboard');
});

// Selecting a card in settings reveals the inline connect form. Local
// providers (Ollama) skip the key input.
settingsCards.forEach((card) => {
	card.addEventListener('click', () => {
		selectedSettingsCard = card;

		settingsCards.forEach((other) => {
			const selected = other === card;

			other.classList.toggle('selected', selected);
			other.setAttribute('aria-pressed', String(selected));
		});

		const local = isLocalProvider(card.dataset.provider);

		settingsLabel.textContent = local ? 'No API key required' : 'API key';
		settingsInput.hidden = local;
		settingsError.hidden = true;

		settingsApply.hidden = !local;
		settingsApply.textContent = local ? 'Connect' : 'Apply';

		settingsConnect.hidden = false;

		(local ? settingsApply : settingsInput).focus();
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

	const local = isLocalProvider(selectedSettingsCard?.dataset.provider);

	if ((!apiKey && !local) || !selectedSettingsCard || settingsApply.disabled) {
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
		await refreshProvidersCache();
	} else {
		settingsError.textContent =
			result.error || 'Could not connect. Check the key and try again.';
		settingsError.hidden = false;
	}

	settingsApply.disabled = false;
	settingsInput.disabled = false;
});

// ---- Chats ----

// Render the provider pill: brand icon of the selected provider + label.
function renderProviderPill() {
	providerPillLabel.textContent = currentProviderName || 'Provider';

	providerPillIcon.replaceChildren();

	const source = document.querySelector(
		`.providers__grid .provider-card[data-provider="${currentProviderName}"] .provider-card__logo`,
	);

	if (currentProviderName && source) {
		providerPillIcon.style.color = getComputedStyle(source).color;
		providerPillIcon.append(
			source.querySelector('svg').cloneNode(true),
		);
	}
}

// Build the floating provider picker menu shown above the composer.
function buildProviderMenu() {
	providerMenu.replaceChildren();

	if (connectedProvidersCache.length === 0) {
		const empty = document.createElement('div');

		empty.className = 'menu__empty';
		empty.textContent = 'No provider connected yet.';

		providerMenu.appendChild(empty);

		return;
	}

	connectedProvidersCache.forEach((provider) => {
		const item = document.createElement('button');

		item.type = 'button';
		item.className = 'menu__item';
		item.setAttribute('role', 'menuitem');
		item.setAttribute(
			'aria-selected',
			String(provider.name === currentProviderName),
		);

		const icon = document.createElement('span');
		icon.className = 'menu__item-icon';

		const source = document.querySelector(
			`.providers__grid .provider-card[data-provider="${provider.name}"] .provider-card__logo`,
		);

		if (source) {
			icon.style.color = getComputedStyle(source).color;
			icon.append(source.querySelector('svg').cloneNode(true));
		}

		const label = document.createElement('span');
		label.textContent = provider.name;

		item.append(icon, label);

		item.addEventListener('click', async () => {
			currentProviderName = provider.name;
			currentModel = '';
			renderProviderPill();
			renderModelPill();
			closeMenus();

			await ensureModelSelected();

			chatTextarea.focus();
		});

		providerMenu.appendChild(item);
	});
}

function openProviderMenu() {
	closeModelMenu();

	buildProviderMenu();

	providerMenu.hidden = false;
	providerPill.setAttribute('aria-expanded', 'true');

	menuBackdrop.hidden = false;
}

function closeProviderMenu() {
	providerMenu.hidden = true;
	providerPill.setAttribute('aria-expanded', 'false');
}

function openModelMenu() {
	closeProviderMenu();

	modelMenu.hidden = false;
	modelPill.setAttribute('aria-expanded', 'true');

	menuBackdrop.hidden = false;

	modelSearch.value = '';

	if (!currentProviderName) {
		modelChoices = [];

		renderModelList('Select a provider first.');

		return;
	}

	renderModelList('Loading models…');

	window.kairo
		.listModels(currentProviderName)
		.then((result) => {
			// The menu might have been closed while loading.
			if (modelMenu.hidden) {
				return;
			}

			if (!result.ok) {
				modelChoices = [];

				renderModelList(result.error || 'Could not load models.');

				return;
			}

			modelChoices = result.models;

			renderModelList(modelChoices.length === 0 ? 'No models found.' : null);

			modelSearch.focus();
		})
		.catch(() => {
			if (modelMenu.hidden) {
				return;
			}

			modelChoices = [];

			renderModelList('Could not load models.');
		});
}

// Render the model list; emptyText replaces the list with a status message.
function renderModelList(emptyText) {
	modelMenuList.replaceChildren();

	if (emptyText) {
		const empty = document.createElement('div');

		empty.className = 'menu__empty';
		empty.textContent = emptyText;

		modelMenuList.appendChild(empty);

		return;
	}

	const query = modelSearch.value.trim().toLowerCase();

	const choices = modelChoices.filter((model) =>
		model.toLowerCase().includes(query),
	);

	if (choices.length === 0) {
		const empty = document.createElement('div');

		empty.className = 'menu__empty';
		empty.textContent = 'No models match your search.';

		modelMenuList.appendChild(empty);

		return;
	}

	choices.forEach((model) => {
		const item = document.createElement('button');

		item.type = 'button';
		item.className = 'menu__item';
		item.setAttribute('role', 'menuitem');
		item.setAttribute('aria-selected', String(model === currentModel));

		const label = document.createElement('span');
		label.textContent = model;

		item.appendChild(label);

		item.addEventListener('click', () => {
			currentModel = model;
			renderModelPill();
			closeMenus();
			chatError.hidden = true;
			chatTextarea.focus();
		});

		modelMenuList.appendChild(item);
	});
}

function closeModelMenu() {
	modelMenu.hidden = true;
	modelPill.setAttribute('aria-expanded', 'false');
}

modelSearch.addEventListener('input', () => {
	renderModelList();
});

function renderModelPill() {
	modelPillLabel.textContent = currentModel || 'Model';
}

// Auto-pick the provider's first model so the composer always has one.
// A silent missing model would make the send path do nothing.
async function ensureModelSelected() {
	if (currentModel || !currentProviderName) {
		return;
	}

	const providerAtRequest = currentProviderName;

	try {
		const result = await window.kairo.listModels(providerAtRequest);

		if (!result.ok || result.models.length === 0) {
			return;
		}

		if (currentProviderName !== providerAtRequest || currentModel) {
			// The provider changed or a model was picked meanwhile.
			return;
		}

		currentModel = result.models[0];
		renderModelPill();
	} catch {
		// Submit shows an error and opens the model menu in this case.
	}
}

function closeMenus() {
	closeProviderMenu();
	closeModelMenu();

	menuBackdrop.hidden = true;
}

// Clicking the backdrop (anywhere above the app, below the menus) closes the
// open menus — even over drag regions, where regular clicks get swallowed.
menuBackdrop.addEventListener('click', () => {
	closeMenus();
});

// Toggle the menus from their pills; opening one closes the other.
providerPill.addEventListener('click', () => {
	if (providerMenu.hidden) {
		closeModelMenu();
		openProviderMenu();

		return;
	}

	closeMenus();
});

modelPill.addEventListener('click', () => {
	if (modelMenu.hidden) {
		closeProviderMenu();
		openModelMenu();

		return;
	}

	closeMenus();
});

// Close the menus on outside clicks or Escape.
document.addEventListener('click', (event) => {
	if (providerMenu.hidden && modelMenu.hidden) {
		return;
	}

	if (
		event.target.closest('#provider-menu') ||
		event.target.closest('#provider-pill') ||
		event.target.closest('#model-menu') ||
		event.target.closest('#model-pill')
	) {
		return;
	}

	closeMenus();
});

document.addEventListener('keydown', (event) => {
	if (event.key === 'Escape' && (!providerMenu.hidden || !modelMenu.hidden)) {
		closeMenus();
		chatTextarea.focus();
	}
});

// Re-fetch the connected providers and refresh the pill/menu source.
async function refreshProvidersCache() {
	const previousProvider = currentProviderName;

	let result = { ok: false, providers: [] };

	try {
		result = await window.kairo.listProviders();
	} catch {
		// Keep the stale cache on failure.
	}

	connectedProvidersCache = result.ok
		? result.providers
		: connectedProvidersCache;

	if (connectedProvidersCache.length === 0) {
		currentProviderName = '';
	} else if (!connectedProvidersCache.some((p) => p.name === currentProviderName)) {
		currentProviderName = connectedProvidersCache[0].name;
	}

	if (currentProviderName !== previousProvider) {
		currentModel = '';
	}

	renderProviderPill();
	renderModelPill();

	return connectedProvidersCache;
}

function chatListItem(chat) {
	const item = document.createElement('li');

	const button = document.createElement('button');

	button.type = 'button';
	button.className = 'chat-list__item';
	button.dataset.chatId = String(chat.id);
	button.textContent = chat.title;
	button.title = chat.title;

	button.addEventListener('click', () => {
		openChat(chat.id);
	});

	item.appendChild(button);

	return item;
}

async function refreshChatList() {
	let result = { ok: false, chats: [] };

	try {
		result = await window.kairo.listChats();
	} catch {
		// Rendered as the empty state below.
	}

	if (!result.ok || result.chats.length === 0) {
		const empty = document.createElement('li');

		empty.className = 'chat-list__empty';
		empty.textContent = result.ok ? 'No chats yet' : 'Could not load chats';

		chatListEl.replaceChildren(empty);

		return;
	}

	chatListEl.replaceChildren(...result.chats.map(chatListItem));

	setChatActive(currentChatId);
}

function setChatActive(id) {
	chatListEl.querySelectorAll('.chat-list__item').forEach((item) => {
		item.classList.toggle(
			'active',
			item.dataset.chatId === String(id),
		);
	});
}

function clearChatMessages() {
	chatColumn.replaceChildren(chatBlank);
	chatBlank.hidden = false;
}

function appendCard(role, content) {
	chatBlank.hidden = true;

	const message = document.createElement('div');

	message.className = `msg msg--${role}`;

	if (role === 'assistant') {
		const avatar = document.createElement('span');

		avatar.className = 'msg__avatar';
		avatar.setAttribute('aria-hidden', 'true');
		avatar.innerHTML =
			'<img src="logo.svg" alt="" draggable="false" />';

		message.appendChild(avatar);
	}

	const text = document.createElement('div');

	text.className = 'msg__text';
	text.textContent = content;

	message.appendChild(text);

	chatColumn.appendChild(message);

	chatMessagesEl.scrollTop = chatMessagesEl.scrollHeight;

	return message;
}

async function openChat(id) {
	currentChatId = Number(id);

	clearChatMessages();
	setChatActive(currentChatId);

	showScreen('screen-chat');

	await refreshProvidersCache();
	await ensureModelSelected();

	let result = { ok: false, messages: [] };

	try {
		result = await window.kairo.getChatMessages(currentChatId);
	} catch {
		// Handled below.
	}

	if (result.ok && result.messages.length > 0) {
		result.messages.forEach((message) => {
			appendCard(message.role, message.content);
		});
	}

	syncSendState();
	chatTextarea.focus();
}

// Create a fresh chat and open it.
newChatButton.addEventListener('click', async () => {
	if (newChatButton.disabled) {
		return;
	}

	newChatButton.disabled = true;

	let result;

	try {
		result = await window.kairo.createChat();
	} catch {
		result = { ok: false };
	}

	newChatButton.disabled = false;

	if (!result.ok) {
		return;
	}

	currentChatId = result.chat.id;

	clearChatMessages();
	showScreen('screen-chat');

	await refreshChatList();
	setChatActive(currentChatId);
	await ensureModelSelected();

	syncSendState();
	chatTextarea.focus();
});

// Send the composer message to the selected provider.
chatComposer.addEventListener('submit', async (event) => {
	event.preventDefault();

	const content = chatTextarea.value.trim();
	const provider = currentProviderName;
	const model = currentModel;

	if (!currentChatId || chatSendButton.disabled) {
		return;
	}

	// Empty input does nothing; the send button is disabled anyway.
	if (!content) {
		return;
	}

	// A missing provider/model must be visible to the user, never silently
	// swallowed.
	if (!provider || !model) {
		chatError.textContent = !provider
			? 'Choose a connected provider first.'
			: 'Choose a model for this provider first.';

		chatError.hidden = false;

		closeProviderMenu();
		openModelMenu();

		return;
	}

	chatSendButton.disabled = true;
	chatTextarea.disabled = true;
	chatError.hidden = true;

	appendCard('user', content);

	chatTextarea.value = '';
	autogrowTextarea();
	syncSendState();

	let result;

	try {
		result = await window.kairo.sendMessage({
			chatId: currentChatId,
			provider,
			model,
			content,
		});
	} catch {
		result = { ok: false, error: 'Send failed. Try again.' };
	}

	if (result.ok) {
		appendCard('assistant', result.result.assistant_message.content);

		// The backend derives the chat title from the first message; refresh
		// the sidebar so the name matches.
		await refreshChatList();
	} else {
		chatError.textContent =
			result.error || 'Could not send the message.';
		chatError.hidden = false;
	}

	chatTextarea.disabled = false;
	chatTextarea.focus();
	syncSendState();
});

// Enter sends, Shift+Enter adds a line break.
chatTextarea.addEventListener('keydown', (event) => {
	if (event.key === 'Enter' && !event.shiftKey) {
		event.preventDefault();

		chatComposer.requestSubmit();
	}
});

// Grow the textarea with its content (capped) and sync the send button.
function autogrowTextarea() {
	chatTextarea.style.height = 'auto';

	const next = Math.min(chatTextarea.scrollHeight, 160);

	chatTextarea.style.height = `${next}px`;
}

function syncSendState() {
	chatSendButton.disabled = chatTextarea.value.trim() === '';
}

chatTextarea.addEventListener('input', () => {
	syncSendState();
	autogrowTextarea();
	chatError.hidden = true;
});
