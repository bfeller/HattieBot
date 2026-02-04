<?php

declare(strict_types=1);

namespace OCA\HattieBridge\Listener;

use OCA\Talk\Events\ChatMessageSentEvent;
use OCA\Talk\Model\Attendee;
use OCP\EventDispatcher\Event;
use OCP\EventDispatcher\IEventListener;

/**
 * Forwards chat messages to HattieBot webhook when the Hattie user is a participant.
 * Skips messages from the Hattie user itself to avoid echo.
 * Config is read from data/hattiebridge-config.json (PHP getenv often doesn't see Docker env).
 *
 * @template-implements IEventListener<ChatMessageSentEvent>
 */
class ChatMessageSentListener implements IEventListener {

	public function __construct() {
		self::staticLog('ChatMessageSentListener::__construct() called');
	}

	public function handle(Event $event): void {
		self::staticLog('ChatMessageSentListener::handle() ENTRY');
		try {
			$this->handleInternal($event);
		} catch (\Throwable $e) {
			$this->debugLog('handle exception: ' . $e->getMessage());
			\OC::$server->getLogger()->warning('HattieBridge: ' . $e->getMessage());
			// Do not rethrow - allow the chat message to be saved even if forwarding fails
		}
		self::staticLog('ChatMessageSentListener::handle() EXIT');
	}

	private static function staticLog(string $msg): void {
		$base = (class_exists(\OC::class) && isset(\OC::$SERVERROOT)) ? \OC::$SERVERROOT : '/var/www/html';
		$path = rtrim((string)$base, '/') . '/data/hattiebridge-debug.log';
		@file_put_contents($path, date('c') . ' [LISTENER] ' . $msg . "\n", FILE_APPEND | LOCK_EX);
	}

	private function loadConfig(): array {
		$config = \OC::$server->getConfig();
		$dataDir = $config->getSystemValue('datadirectory', \OC::$SERVERROOT . '/data') ?: (\OC::$SERVERROOT . '/data');
		$path = rtrim($dataDir, '/') . '/hattiebridge-config.json';
		if (is_file($path) && is_readable($path)) {
			$json = file_get_contents($path);
			if ($json !== false) {
				$decoded = json_decode($json, true);
				if (is_array($decoded)) {
					return $decoded;
				}
			}
		}
		return [
			'webhook_url' => getenv('HATTIEBOT_WEBHOOK_URL') ?: '',
			'webhook_secret' => getenv('HATTIEBOT_WEBHOOK_SECRET') ?: '',
			'hattie_user' => getenv('HATTIEBOT_HATTIE_USER') ?: '',
		];
	}

	private function handleInternal(Event $event): void {
		$this->debugLog('handleInternal: event=' . ($event instanceof ChatMessageSentEvent ? 'ChatMessageSentEvent' : get_class($event)));
		if (!$event instanceof ChatMessageSentEvent) {
			$this->debugLog('handleInternal: skip, not ChatMessageSentEvent');
			return;
		}

		$cfg = $this->loadConfig();
		$hattieUserId = $this->sanitizeUserId($cfg['hattie_user'] ?? '');
		$webhookUrl = $cfg['webhook_url'] ?? '';
		$webhookSecret = $cfg['webhook_secret'] ?? '';

		if ($hattieUserId === '' || empty($webhookUrl) || empty($webhookSecret)) {
			$this->debugLog('skip: config missing hattie_user=' . ($hattieUserId ?: 'empty') . ' url=' . ($webhookUrl ? 'set' : 'empty'));
			return;
		}

		$room = $event->getRoom();
		$comment = $event->getComment();
		$this->debugLog('handleInternal: room=' . $room->getToken() . ' verb=' . $comment->getVerb());

		// Skip non-comment messages (system messages, etc.)
		if ($comment->getVerb() !== 'comment') {
			$this->debugLog('handleInternal: skip, verb!==comment');
			return;
		}

		// Skip if message is from Hattie user (avoid echo)
		$actorType = $comment->getActorType();
		$actorId = $comment->getActorId();
		$this->debugLog('handleInternal: actor=' . $actorType . '/' . $actorId . ' hattie=' . $hattieUserId);
		if ($actorType === Attendee::ACTOR_USERS && $actorId === $hattieUserId) {
			$this->debugLog('handleInternal: skip, message from hattie (avoid echo)');
			return;
		}

		// Check if Hattie user is a participant in this room
		// Try "users/hattie" first (Talk v4+ format), then "hattie"
		$participant = null;
		foreach ([Attendee::ACTOR_USERS . '/' . $hattieUserId, $hattieUserId] as $participantId) {
			try {
				$participant = $room->getParticipant($participantId);
				break;
			} catch (\OCA\Talk\Exceptions\ParticipantNotFoundException $e) {
				// Fallback to next format
			}
		}
		if ($participant === null) {
			$this->debugLog('skip: user ' . $hattieUserId . ' not in room ' . $room->getToken());
			return;
		}

		// Build payload in Activity Streams 2.0 format (same as Talk bot webhook)
		$actorIdPrefixed = $actorType . '/' . $actorId;
		$actorDisplayName = $this->getActorDisplayName($event);
		$content = json_encode([
			'message' => $comment->getMessage(),
			'parameters' => method_exists($comment, 'getMessageParameters') ? ($comment->getMessageParameters() ?? []) : [],
		]);

		$payload = [
			'type' => 'Create',
			'actor' => [
				'id' => $actorIdPrefixed,
				'name' => $actorDisplayName,
			],
			'object' => [
				'id' => (string) $comment->getId(),
				'name' => 'message',
				'content' => $content,
				'mediaType' => 'text/markdown',
			],
			'target' => [
				'id' => $room->getToken(),
				'name' => $room->getDisplayName($hattieUserId),
			],
		];

		$body = json_encode($payload);
		if ($body === false) {
			\OC::$server->getLogger()->error('HattieBridge: failed to encode payload');
			return;
		}

		$ch = curl_init($webhookUrl);
		if ($ch === false) {
			\OC::$server->getLogger()->error('HattieBridge: curl_init failed');
			return;
		}

		curl_setopt_array($ch, [
			CURLOPT_POST => true,
			CURLOPT_POSTFIELDS => $body,
			CURLOPT_HTTPHEADER => [
				'Content-Type: application/json',
				'X-HattieBridge-Secret: ' . $webhookSecret,
			],
			CURLOPT_RETURNTRANSFER => true,
			CURLOPT_TIMEOUT => 10,
		]);

		$response = curl_exec($ch);
		$httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
		$error = curl_error($ch);
		curl_close($ch);

		if ($error) {
			\OC::$server->getLogger()->warning('HattieBridge: webhook request failed: ' . $error);
			return;
		}

		if ($httpCode >= 400) {
			\OC::$server->getLogger()->warning('HattieBridge: webhook returned ' . $httpCode . ': ' . ($response ?: ''));
		} else {
			$this->debugLog('forwarded to webhook room=' . $room->getToken());
		}
	}

	private function debugLog(string $msg): void {
		$config = \OC::$server->getConfig();
		$dataDir = $config->getSystemValue('datadirectory', \OC::$SERVERROOT . '/data') ?: (\OC::$SERVERROOT . '/data');
		$path = rtrim($dataDir, '/') . '/hattiebridge-debug.log';
		@file_put_contents($path, date('c') . ' ' . $msg . "\n", FILE_APPEND | LOCK_EX);
	}

	private function sanitizeUserId(string $name): string {
		return strtolower(preg_replace('/\s+/', '', $name) ?: '');
	}

	private function getActorDisplayName(ChatMessageSentEvent $event): string {
		$participant = $event->getParticipant();
		if ($participant !== null) {
			try {
				$displayName = $participant->getAttendee()->getDisplayName();
				if ($displayName !== '') {
					return $displayName;
				}
			} catch (\Throwable $e) {
				// Fallback below
			}
		}
		$comment = $event->getComment();
		$actorId = $comment->getActorId();
		// Fallback: use actor ID as display name
		return $actorId;
	}
}
