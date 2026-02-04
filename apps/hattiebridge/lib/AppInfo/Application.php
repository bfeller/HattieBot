<?php

declare(strict_types=1);

namespace OCA\HattieBridge\AppInfo;

use OCA\Talk\Events\ChatMessageSentEvent;
use OCA\HattieBridge\Listener\ChatMessageSentListener;
use OCP\AppFramework\App;
use OCP\AppFramework\Bootstrap\IBootContext;
use OCP\AppFramework\Bootstrap\IBootstrap;
use OCP\AppFramework\Bootstrap\IRegistrationContext;

class Application extends App implements IBootstrap {

	public function __construct() {
		parent::__construct('hattiebridge');

		$listenerClass = 'OCA\\HattieBridge\\Listener\\ChatMessageSentListener';
		$autoload = __DIR__ . '/../../vendor/autoload.php';
		$listenerFile = __DIR__ . '/../Listener/ChatMessageSentListener.php';

		if (is_file($autoload)) {
			require_once $autoload;
		}
		if (!class_exists($listenerClass, false) && file_exists($listenerFile)) {
			require_once $listenerFile;
		}
	}

	public function register(IRegistrationContext $context): void {
		$this->debugLog('Application::register() called - HattieBridge loading');

		// Skip listener if disabled (for debugging 500 errors)
		if (getenv('HATTIEBRIDGE_DISABLED') === '1') {
			$this->debugLog('HattieBridge listener disabled via HATTIEBRIDGE_DISABLED');
			return;
		}

		$listenerClass = 'OCA\\HattieBridge\\Listener\\ChatMessageSentListener';
		$listenerFile = __DIR__ . '/../Listener/ChatMessageSentListener.php';

		// Use string for class_exists to avoid fatal if class not loaded
		if (!class_exists($listenerClass, false)) {
			$this->debugLog('ChatMessageSentListener class not found (file=' . $listenerFile . ' exists=' . (file_exists($listenerFile) ? 'yes' : 'no') . '), skipping listener');
			return;
		}
		$this->debugLog('ChatMessageSentListener class loaded, registering...');

		try {
			$context->registerService($listenerClass, function ($c) {
				$this->debugLog('ChatMessageSentListener factory: instantiating');
				return new ChatMessageSentListener();
			});

			$context->registerEventListener(ChatMessageSentEvent::class, $listenerClass);
			$this->debugLog('HattieBridge listener registered successfully');
		} catch (\Throwable $e) {
			$this->debugLog('Failed to register listener: ' . $e->getMessage() . ' at ' . $e->getFile() . ':' . $e->getLine());
			// Do not rethrow - allow Talk to work without our listener
		}
	}

	private function debugLog(string $msg): void {
		$base = (class_exists(\OC::class) && isset(\OC::$SERVERROOT)) ? \OC::$SERVERROOT : dirname(__DIR__, 3);
		$path = rtrim((string)$base, '/') . '/data/hattiebridge-debug.log';
		@file_put_contents($path, date('c') . ' [APP] ' . $msg . "\n", FILE_APPEND | LOCK_EX);
	}

	public function boot(IBootContext $context): void {
		$this->debugLog('Application::boot() called - HattieBridge booted');
	}
}
