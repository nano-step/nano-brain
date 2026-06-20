class NotificationService
  def self.notify(event, payload = nil)
    Rails.logger.info("Event: #{event}")
  end
end
