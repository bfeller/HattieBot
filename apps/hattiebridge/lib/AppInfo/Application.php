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

		// Include composer autoloader for PSR-4 class loading
		$autoloadFile = __DIR__ . '/../../vendor/autoload.php';
		if (file_exists($autoloadFile)) {
			include_once $autoloadFile;
		}
	}

	public function register(IRegistrationContext $context): void {
		$this->debugLog('Application::register() called - HattieBridge loading');

		// Register the listener class in the container so DI can instantiate it
		$context->registerService(ChatMessageSentListener::class, function ($c) {
			return new ChatMessageSentListener();
		});

		$context->registerEventListener(ChatMessageSentEvent::class, ChatMessageSentListener::class);
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
