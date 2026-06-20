class TokenGenerator
  def self.generate(email, password)
    Digest::SHA256.hexdigest("#{email}#{password}")
  end

  def self.validate(token)
    token.present? && token.length > 16
  end
end
