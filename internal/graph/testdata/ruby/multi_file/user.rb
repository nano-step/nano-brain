class User < ApplicationRecord
  def full_name
    "#{first_name} #{last_name}"
  end

  def active?
    status == "active"
  end
end
