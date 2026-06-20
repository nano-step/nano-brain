class User < ApplicationRecord
  validates :name, presence: true
  validates :email, presence: true, uniqueness: true

  def full_name
    "#{first_name} #{last_name}"
  end

  def self.find_by_email(email)
    where(email: email).first
  end

  def deactivate
    update(active: false)
    send_deactivation_email
  end

  private

  def send_deactivation_email
    UserMailer.deactivation(self).deliver_later
  end
end
